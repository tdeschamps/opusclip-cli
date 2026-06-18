package clips

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/text"
)

// downloadResult is one entry in the JSON manifest emitted on --json.
type downloadResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func newDownloadCmd(f *cmdutil.Factory) *cobra.Command {
	var project, outDir, clipID string
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download a project's generated clips as mp4 files",
		Long: `Download the rendered mp4 for each clip in a project.

  opusclip clips download --project P123 --out ./clips
  opusclip clips download --project P123 --clip P123.c1`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			items, err := client.ListExportableClips(cmd.Context(), project, 0)
			if err != nil {
				return err
			}
			items = selectClips(items, clipID)
			if len(items) == 0 {
				return cmdutil.NewSilentError(cmdutil.ExitNotFound,
					fmt.Errorf("no downloadable clips for project %s", project))
			}
			return downloadClips(cmd.Context(), f, items, outDir)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&outDir, "out", ".", "Directory to write clips into")
	cmd.Flags().StringVar(&clipID, "clip", "", "Download only this clip ID (default: all)")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// selectClips filters to a single clip when clipID is set, otherwise returns all.
func selectClips(items []api.ExportableClip, clipID string) []api.ExportableClip {
	if clipID == "" {
		return items
	}
	for _, c := range items {
		if c.ID == clipID {
			return []api.ExportableClip{c}
		}
	}
	return nil
}

// gcsClient downloads the signed export URLs. It deliberately does NOT carry the
// API bearer token (the GCS URLs are pre-signed; sending auth would be wrong).
var gcsClient = &http.Client{Timeout: 0}

func downloadClips(ctx context.Context, f *cmdutil.Factory, items []api.ExportableClip, outDir string) error {
	io := f.IOStreams
	results := make([]downloadResult, 0, len(items))

	if !f.Flags.DryRun {
		if err := os.MkdirAll(outDir, 0o750); err != nil {
			return err
		}
	}

	for _, c := range items {
		if c.URIForExport == "" {
			io.Errf("%s clip %s has no export URL yet; skipping\n", io.WarnIcon(), c.ID)
			continue
		}
		dest := filepath.Join(outDir, clipFilename(c))
		if f.Flags.DryRun {
			io.Errf("would download %s → %s\n", c.ID, dest)
			results = append(results, downloadResult{ID: c.ID, Title: c.Title, Path: dest})
			continue
		}

		sp := io.NewSpinner("Downloading " + c.ID + "…")
		sp.Start()
		n, err := downloadFile(ctx, c.URIForExport, dest)
		sp.Stop()
		if err != nil {
			return fmt.Errorf("download %s: %w", c.ID, err)
		}
		io.Errf("%s %s (%s)\n", io.SuccessIcon(), dest, text.HumanBytes(n))
		results = append(results, downloadResult{ID: c.ID, Title: c.Title, Path: dest, Bytes: n})
	}

	// On a structured-output request, emit the manifest to stdout.
	_, err := cmdutil.PrintStructured(f, results)
	return err
}

// downloadFile streams url to dest and returns the bytes written. A failed close
// is surfaced as an error so a truncated/unflushed file isn't reported as success.
func downloadFile(ctx context.Context, url, dest string) (n int64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := gcsClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	n, err = io.Copy(out, resp.Body)
	return n, err
}

var unsafeFilename = regexp.MustCompile(`[^\w.-]+`)

// clipFilename builds a safe ".mp4" filename from the clip title (falling back
// to its ID).
func clipFilename(c api.ExportableClip) string {
	base := strings.TrimSpace(c.Title)
	if base == "" {
		base = c.ID
	}
	base = unsafeFilename.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-.")
	if base == "" {
		base = c.ID
	}
	const maxLen = 80
	if len(base) > maxLen {
		base = base[:maxLen]
	}
	return base + ".mp4"
}
