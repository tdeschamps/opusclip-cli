//go:build contract

// Package contract holds the gated nightly smoke suite that runs against the
// real OpusClip API (it is never part of the per-PR path: no secrets for forks,
// no flakiness gate). It validates that live response shapes still satisfy our
// decoders. Run with: go test -tags=contract ./... -run Contract
package contract

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/config"
	"github.com/tdeschamps/opusclip-cli/internal/httpclient"
)

func liveClient(t *testing.T) *api.Client {
	t.Helper()
	key := os.Getenv("OPUSCLIP_API_KEY")
	if key == "" {
		t.Skip("OPUSCLIP_API_KEY not set; skipping contract tests")
	}
	base := os.Getenv("OPUSCLIP_BASE_URL")
	if base == "" {
		base = config.DefaultBaseURL
	}
	hc := httpclient.New(httpclient.Options{
		Token:      func() (string, error) { return key, nil },
		MaxRetries: 2,
	})
	return api.New(api.Options{
		BaseURL:    base,
		HTTPClient: hc,
		OrgID:      os.Getenv("OPUSCLIP_ORG_ID"),
	})
}

// TestContractValidate confirms the credential is accepted by the live API
// (the same probe `auth login` / `doctor` use).
func TestContractValidate(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := c.Validate(ctx); err != nil {
		t.Fatalf("credential validation failed: %v", err)
	}
}

// TestContractExportableClipsDecode confirms the exportable-clips list still
// decodes into our model. It needs OPUSCLIP_CONTRACT_PROJECT_ID (a project in
// the sandbox with at least one clip); without it the decode check is skipped.
func TestContractExportableClipsDecode(t *testing.T) {
	c := liveClient(t)
	projectID := os.Getenv("OPUSCLIP_CONTRACT_PROJECT_ID")
	if projectID == "" {
		t.Skip("OPUSCLIP_CONTRACT_PROJECT_ID not set; skipping clip decode check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clips, err := c.ListExportableClips(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("exportable-clips decode failed: %v", err)
	}
	if len(clips) > 0 && clips[0].ID == "" {
		t.Errorf("decoded clip missing id: %+v", clips[0])
	}
}
