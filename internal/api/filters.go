package api

import (
	"net/url"
	"strconv"
)

// DefaultPageSize is the page size used when walking bare-array list endpoints
// (e.g. exportable-clips). Exported so the raw `api --paginate` escape hatch
// paginates the same way the typed list methods do.
const DefaultPageSize = 50

// FirstPageNum is the index of the first page. The OpenAPI spec documents
// pageNum as "started with 1"; kept as a single named constant so one edit
// covers every paginating path should the live API turn out 0-based.
const FirstPageNum = 1

// ClipFilter selects exportable clips for a project. Limit caps the total
// returned (0 = unbounded).
type ClipFilter struct {
	ProjectID string
	Limit     int
}

// query builds the query string for a given absolute page number and page size.
func (f ClipFilter) query(page, size int) url.Values {
	return url.Values{
		"q":         {"findByProjectId"},
		"projectId": {f.ProjectID},
		"pageNum":   {strconv.Itoa(page)},
		"pageSize":  {strconv.Itoa(size)},
	}
}
