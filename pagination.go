package loops

import "iter"

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
// leave PerPage at zero to use the API default page size, and leave Cursor
// empty to request the first page.
type PaginationParams struct {
	PerPage int
	Cursor  string
}

// Paginate fetches every page from a paginated list endpoint by repeatedly
// calling fetch with the previous page's NextCursor, and returns all items
// concatenated together. Use it with list methods like
// [Client.ListTransactionals] and [Client.ListCampaigns]:
//
//	all, err := loops.Paginate(func(cursor string) ([]loops.Transactional, *loops.Pagination, error) {
//	    return client.ListTransactionals(ctx, loops.ListTransactionalsParams{
//	        PaginationParams: loops.PaginationParams{Cursor: cursor},
//	    })
//	})
func Paginate[T any](fetch func(cursor string) ([]T, *Pagination, error)) ([]T, error) {
	var all []T
	for item, err := range PaginateSeq(fetch) {
		if err != nil {
			return nil, err
		}
		all = append(all, item)
	}
	return all, nil
}

// PaginateSeq returns an iterator over every item from a paginated list
// endpoint. It calls fetch for the next page only after the items of the
// current page are consumed, so a loop that stops early does not fetch the
// remaining pages. If fetch returns an error, the iterator yields that error
// with a zero item and stops:
//
//	seq := loops.PaginateSeq(func(cursor string) ([]loops.Transactional, *loops.Pagination, error) {
//	    return client.ListTransactionals(ctx, loops.ListTransactionalsParams{
//	        PaginationParams: loops.PaginationParams{Cursor: cursor},
//	    })
//	})
//	for item, err := range seq {
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Println(item.ID)
//	}
func PaginateSeq[T any](fetch func(cursor string) ([]T, *Pagination, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		cursor := ""
		for {
			items, page, err := fetch(cursor)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if page.NextCursor == "" {
				return
			}
			cursor = page.NextCursor
		}
	}
}
