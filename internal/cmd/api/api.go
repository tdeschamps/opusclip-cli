// Package api implements `opusclip api` — the raw authenticated request escape
// hatch (like `gh api` / `stripe get`). Auth, base URL, and pagination are
// handled; the response is raw JSON by default.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	opusapi "github.com/tdeschamps/opusclip-cli/internal/api"
	"github.com/tdeschamps/opusclip-cli/internal/cmdutil"
	"github.com/tdeschamps/opusclip-cli/internal/output"
)

// NewCmdAPI returns the api command.
func NewCmdAPI(f *cmdutil.Factory) *cobra.Command {
	var params []string
	var fields []string
	var input string
	var paginate bool

	cmd := &cobra.Command{
		Use:     "api <method> <path>",
		Short:   "Make an authenticated request to any OpusClip API endpoint",
		GroupID: "core",
		Long: `Make an authenticated request to an arbitrary OpusClip REST endpoint.

  opusclip api GET /api/clip-projects/P123
  opusclip api POST /api/clip-projects --field videoUrl=https://youtu.be/x
  opusclip api GET /api/exportable-clips --param q=findByProjectId --param projectId=P123 --paginate
  cat body.json | opusclip api POST /api/clip-projects --input -`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := "/"
			switch {
			case len(args) == 2:
				path = args[1]
			case !isHTTPMethod(method):
				// "opusclip api /calls" shorthand → GET.
				path, method = args[0], "GET"
			}

			client, err := f.APIClient()
			if err != nil {
				return err
			}

			query := url.Values{}
			for _, p := range params {
				k, v, ok := strings.Cut(p, "=")
				if !ok {
					return cmdutil.NewUsageError(fmt.Errorf("--param must be key=value, got %q", p))
				}
				query.Add(k, v)
			}

			body, err := buildBody(f, fields, input)
			if err != nil {
				return err
			}

			p, err := f.Printer()
			if err != nil {
				return err
			}

			if paginate {
				return paginateAll(cmd, client, method, path, query, body, p)
			}

			raw, err := client.Raw(cmd.Context(), method, path, query, body)
			if err != nil {
				return err
			}
			return printRaw(f, p, raw)
		},
	}
	cmd.Flags().StringArrayVar(&params, "param", nil, "Query parameter key=value (repeatable)")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "JSON body field key=value (repeatable)")
	cmd.Flags().StringVar(&input, "input", "", "Read the request body from a file (- for stdin)")
	cmd.Flags().BoolVar(&paginate, "paginate", false, "Follow pages and concatenate results")
	return cmd
}

func buildBody(f *cmdutil.Factory, fields []string, input string) ([]byte, error) {
	if input != "" {
		if input == "-" {
			return f.IOStreams.ReadAllStdin()
		}
		// The request body path is explicitly supplied by the user via --input.
		return os.ReadFile(input)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	m := map[string]any{}
	for _, fld := range fields {
		k, v, ok := strings.Cut(fld, "=")
		if !ok {
			return nil, cmdutil.NewUsageError(fmt.Errorf("--field must be key=value, got %q", fld))
		}
		m[k] = v
	}
	return json.Marshal(m)
}

// paginateAll walks OpusClip's bare-array list responses (e.g. exportable-clips)
// using pageNum/pageSize and prints the concatenated rows as one JSON array. It
// stops on a short or empty page. It reuses the api package's pagination
// constants so the escape hatch pages the same way the typed list methods do.
func paginateAll(cmd *cobra.Command, client *opusapi.Client, method, path string, query url.Values, body []byte, p *output.Printer) error {
	var all []json.RawMessage
	// Clone the caller's query once; each page only rewrites the page params.
	q := maps.Clone(query)
	q.Set("pageSize", strconv.Itoa(opusapi.DefaultPageSize))
	for page := opusapi.FirstPageNum; ; page++ {
		q.Set("pageNum", strconv.Itoa(page))
		raw, err := client.Raw(cmd.Context(), method, path, q, body)
		if err != nil {
			return err
		}
		var pageItems []json.RawMessage
		if err := json.Unmarshal(raw, &pageItems); err != nil {
			return fmt.Errorf("response is not a JSON array (only bare-array list endpoints support --paginate): %w", err)
		}
		all = append(all, pageItems...)
		if len(pageItems) < opusapi.DefaultPageSize {
			break
		}
	}
	return p.PrintJSON(all)
}

func printRaw(f *cmdutil.Factory, p *output.Printer, raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		_, err := f.IOStreams.Out.Write(raw)
		return err
	}
	return p.PrintJSON(v)
}

func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		return true
	}
	return false
}
