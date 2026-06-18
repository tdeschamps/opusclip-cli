// Package api is the OpusClip REST adapter: typed models and a thin Client over
// the documented v0.1 endpoints (clip projects + exportable clips), plus a Raw
// escape hatch. Auth is normally injected by the httpclient chain; the Client
// adds the optional x-opus-org-id header and maps non-2xx responses to *Error.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Options configures a Client.
type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	// OrgID, when set, is sent as the x-opus-org-id header (multi-org users).
	OrgID string
	// Token is used only when HTTPClient does not already inject auth (the
	// httpclient chain normally handles it). Kept for direct/test usage.
	Token func() (string, error)
}

// Client talks to the OpusClip REST API.
type Client struct {
	baseURL string
	http    *http.Client
	token   func() (string, error)
	orgID   string
}

// New constructs a Client.
func New(opt Options) *Client {
	hc := opt.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		baseURL: strings.TrimRight(opt.BaseURL, "/"),
		http:    hc,
		token:   opt.Token,
		orgID:   opt.OrgID,
	}
}

// doJSON performs a request and decodes a JSON response into out.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body io.Reader, out any) error {
	raw, err := c.do(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader) ([]byte, error) {
	u := c.baseURL + ensureLeadingSlash(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.orgID != "" {
		req.Header.Set("x-opus-org-id", c.orgID)
	}
	// If the http.Client doesn't inject auth, do it here.
	if c.token != nil && req.Header.Get("Authorization") == "" {
		tok, err := c.token()
		if err != nil {
			return nil, err
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newAPIError(resp.StatusCode, resp.Header, data)
	}
	return data, nil
}

// Raw performs an arbitrary authenticated request (used by `opusclip api`).
func (c *Client) Raw(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, error) {
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	return c.do(ctx, method, path, query, r)
}

// CreateProject submits a video for clipping (POST /api/clip-projects).
func (c *Client) CreateProject(ctx context.Context, in CreateProjectInput) (Project, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return Project{}, err
	}
	var out Project
	err = c.doJSON(ctx, http.MethodPost, "/api/clip-projects", nil, bytes.NewReader(body), &out)
	return out, err
}

// GetProject fetches a project's current state (GET /api/clip-projects/{id}).
func (c *Client) GetProject(ctx context.Context, projectID string) (Project, error) {
	var out Project
	err := c.doJSON(ctx, http.MethodGet, "/api/clip-projects/"+url.PathEscape(projectID), nil, nil, &out)
	return out, err
}

// ListExportableClips returns a project's generated clips, walking the paged
// bare-array response (GET /api/exportable-clips). limit caps the total (0 =
// all). It stops on a short page or when the limit is reached. When a small
// limit is set it shrinks the page size so a `--limit 3` doesn't pull a full
// page just to discard most of it.
func (c *Client) ListExportableClips(ctx context.Context, projectID string, limit int) ([]ExportableClip, error) {
	f := ClipFilter{ProjectID: projectID, Limit: limit}
	var out []ExportableClip
	for page := FirstPageNum; ; page++ {
		size := DefaultPageSize
		if limit > 0 {
			if remaining := limit - len(out); remaining < size {
				size = remaining
			}
		}
		var pageItems []ExportableClip
		if err := c.doJSON(ctx, http.MethodGet, "/api/exportable-clips", f.query(page, size), nil, &pageItems); err != nil {
			return nil, err
		}
		out = append(out, pageItems...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
		// A page shorter than what we asked for means the server has no more.
		if len(pageItems) < size {
			return out, nil
		}
	}
}

// Validate confirms the active credential is accepted by the API. OpusClip has
// no identity endpoint, so it probes the exportable-clips list with a throwaway
// project id: a 401 means the key is bad; any other status (including 4xx for
// the bogus id, or a 403 monthly-cap) means auth succeeded.
func (c *Client) Validate(ctx context.Context) error {
	f := ClipFilter{ProjectID: "validate-probe"}
	_, err := c.do(ctx, http.MethodGet, "/api/exportable-clips", f.query(FirstPageNum, 1), nil)
	if err == nil {
		return nil
	}
	var apiErr *Error
	if asError(err, &apiErr) {
		if apiErr.StatusCode == http.StatusUnauthorized {
			return err // genuine auth failure
		}
		return nil // reached an authenticated handler; key is valid
	}
	return err // transport/network error — let the caller decide
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}
