package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestCreateProjectSendsBodyAndHeaders(t *testing.T) {
	var gotBody CreateProjectInput
	var gotAuth, gotOrg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotOrg = r.Header.Get("x-opus-org-id")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"projectId":"P1","stage":"PENDING"}`))
	}))
	defer srv.Close()

	c := New(Options{
		BaseURL: srv.URL,
		OrgID:   "org_9",
		Token:   func() (string, error) { return "tok", nil },
	})
	p, err := c.CreateProject(context.Background(), CreateProjectInput{VideoURL: "https://v"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ProjectID != "P1" || p.Stage != StagePending {
		t.Errorf("project = %+v", p)
	}
	if gotBody.VideoURL != "https://v" {
		t.Errorf("body videoUrl = %q", gotBody.VideoURL)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotOrg != "org_9" {
		t.Errorf("org = %q", gotOrg)
	}
}

func TestGetProjectDecodesStage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/clip-projects/P1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"projectId":"P1","stage":"RENDER","error":""}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	p, err := c.GetProject(context.Background(), "P1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Stage != StageRender {
		t.Errorf("stage = %q", p.Stage)
	}
}

func TestListExportableClipsPaginates(t *testing.T) {
	// Two full pages then a short page → 50 + 50 + 3 = 103 clips.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("pageNum"))
		n := DefaultPageSize
		if page >= 3 {
			n = 3
		}
		clips := make([]ExportableClip, n)
		for i := range clips {
			clips[i] = ExportableClip{ID: fmt.Sprintf("P1.c%d_%d", page, i)}
		}
		_ = json.NewEncoder(w).Encode(clips)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	got, err := c.ListExportableClips(context.Background(), "P1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2*DefaultPageSize+3 {
		t.Errorf("got %d clips, want %d", len(got), 2*DefaultPageSize+3)
	}
}

func TestListExportableClipsHonorsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clips := make([]ExportableClip, DefaultPageSize)
		_ = json.NewEncoder(w).Encode(clips)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	got, err := c.ListExportableClips(context.Background(), "P1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Errorf("limit not honored: got %d", len(got))
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"ok", http.StatusOK, false},
		{"unauthorized", http.StatusUnauthorized, true},
		{"cap is valid", http.StatusForbidden, false},
		{"bad request is valid auth", http.StatusBadRequest, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.status == http.StatusOK {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			c := New(Options{BaseURL: srv.URL})
			err := c.Validate(context.Background())
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRawEscapeHatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	raw, err := c.Raw(context.Background(), http.MethodGet, "/api/anything", nil, nil)
	if err != nil || string(raw) != `{"ok":true}` {
		t.Errorf("raw = %q err %v", raw, err)
	}
}
