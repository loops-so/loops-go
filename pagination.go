package loops

// Pagination describes the position of a single page within a paginated
// result set. NextCursor is empty when the current page is the last one.
type Pagination struct {
	TotalResults    int    `json:"totalResults"`
	ReturnedResults int    `json:"returnedResults"`
	PerPage         int    `json:"perPage"`
	TotalPages      int    `json:"totalPages"`
	NextCursor      string `json:"nextCursor"`
	NextPage        string `json:"nextPage"`
}

// PaginationParams controls a single page request. Both fields are optional;
// leave Cursor empty to request the first page.
type PaginationParams struct {
	PerPage string
	Cursor  string
}

// Paginate fetches every page from a paginated list endpoint by repeatedly
// calling fetch with the previous page's NextCursor, and returns all items
// concatenated together. Use it with list methods like
// [Client.ListTransactionals] and [Client.ListCampaigns]:
//
//	all, err := loops.Paginate(func(cursor string) ([]loops.Transactional, *loops.Pagination, error) {
//	    return client.ListTransactionals(loops.ListTransactionalsParams{
//	        PaginationParams: loops.PaginationParams{Cursor: cursor},
//	    })
//	})
func Paginate[T any](fetch func(cursor string) ([]T, *Pagination, error)) ([]T, error) {
	var all []T
	cursor := ""
	for {
		items, page, err := fetch(cursor)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if page.NextCursor == "" {
			return all, nil
		}
		cursor = page.NextCursor
	}
}
