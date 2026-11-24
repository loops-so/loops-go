package loops

import (
	"errors"
	"slices"
	"testing"
)

func TestPaginate(t *testing.T) {
	t.Run("single page", func(t *testing.T) {
		calls := 0
		fetch := func(cursor string) ([]string, *Pagination, error) {
			calls++
			return []string{"a", "b"}, &Pagination{NextCursor: ""}, nil
		}

		items, err := Paginate(fetch)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if calls != 1 {
			t.Errorf("expected 1 fetch call, got %d", calls)
		}
	})

	t.Run("multiple pages", func(t *testing.T) {
		pages := []struct {
			items  []string
			cursor string
		}{
			{[]string{"a", "b"}, "cursor1"},
			{[]string{"c", "d"}, "cursor2"},
			{[]string{"e"}, ""},
		}
		call := 0
		fetch := func(cursor string) ([]string, *Pagination, error) {
			p := pages[call]
			call++
			return p.items, &Pagination{NextCursor: p.cursor}, nil
		}

		items, err := Paginate(fetch)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 5 {
			t.Errorf("expected 5 items, got %d", len(items))
		}
		if call != 3 {
			t.Errorf("expected 3 fetch calls, got %d", call)
		}
	})

	t.Run("error on first fetch", func(t *testing.T) {
		fetchErr := errors.New("api error")
		fetch := func(cursor string) ([]string, *Pagination, error) {
			return nil, nil, fetchErr
		}

		_, err := Paginate(fetch)
		if !errors.Is(err, fetchErr) {
			t.Errorf("expected fetch error, got %v", err)
		}
	})

	t.Run("error mid-pagination", func(t *testing.T) {
		fetchErr := errors.New("api error")
		call := 0
		fetch := func(cursor string) ([]string, *Pagination, error) {
			call++
			if call == 2 {
				return nil, nil, fetchErr
			}
			return []string{"a"}, &Pagination{NextCursor: "cursor1"}, nil
		}

		_, err := Paginate(fetch)
		if !errors.Is(err, fetchErr) {
			t.Errorf("expected fetch error, got %v", err)
		}
	})
}

func TestPaginateSeq(t *testing.T) {
	pages := []struct {
		items  []string
		cursor string
	}{
		{[]string{"a", "b"}, "cursor1"},
		{[]string{"c", "d"}, "cursor2"},
		{[]string{"e"}, ""},
	}

	t.Run("multiple pages", func(t *testing.T) {
		var cursors []string
		fetch := func(cursor string) ([]string, *Pagination, error) {
			p := pages[len(cursors)]
			cursors = append(cursors, cursor)
			return p.items, &Pagination{NextCursor: p.cursor}, nil
		}

		var got []string
		for item, err := range PaginateSeq(fetch) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, item)
		}
		if !slices.Equal(got, []string{"a", "b", "c", "d", "e"}) {
			t.Errorf("items = %v, want a through e in order", got)
		}
		if !slices.Equal(cursors, []string{"", "cursor1", "cursor2"}) {
			t.Errorf("cursors = %v, want each page's NextCursor passed to the next fetch", cursors)
		}
	})

	t.Run("early break stops fetching", func(t *testing.T) {
		calls := 0
		fetch := func(cursor string) ([]string, *Pagination, error) {
			p := pages[calls]
			calls++
			return p.items, &Pagination{NextCursor: p.cursor}, nil
		}

		var got []string
		for item, err := range PaginateSeq(fetch) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, item)
			if item == "c" {
				break
			}
		}
		if !slices.Equal(got, []string{"a", "b", "c"}) {
			t.Errorf("items = %v, want a, b, c", got)
		}
		if calls != 2 {
			t.Errorf("expected 2 fetch calls, got %d", calls)
		}
	})

	t.Run("error on later page", func(t *testing.T) {
		fetchErr := errors.New("api error")
		calls := 0
		fetch := func(cursor string) ([]string, *Pagination, error) {
			calls++
			if calls == 2 {
				return nil, nil, fetchErr
			}
			return []string{"a", "b"}, &Pagination{NextCursor: "cursor1"}, nil
		}

		var got []string
		var gotErr error
		for item, err := range PaginateSeq(fetch) {
			if err != nil {
				gotErr = err
				continue
			}
			got = append(got, item)
		}
		if !errors.Is(gotErr, fetchErr) {
			t.Errorf("expected fetch error, got %v", gotErr)
		}
		if !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("items = %v, want the first page's items before the error", got)
		}
		if calls != 2 {
			t.Errorf("expected the iterator to stop after the error, got %d fetch calls", calls)
		}
	})
}
