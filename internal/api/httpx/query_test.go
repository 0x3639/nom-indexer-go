package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantPage int
		wantSize int
	}{
		{"defaults", "", 1, DefaultPageSize},
		{"explicit", "?page=3&page_size=25", 3, 25},
		{"clamps over max", "?page=1&page_size=10000", 1, MaxPageSize},
		{"ignores negative page", "?page=-1&page_size=10", 1, 10},
		{"ignores zero page_size", "?page=2&page_size=0", 2, DefaultPageSize},
		{"ignores non-numeric", "?page=abc&page_size=xyz", 1, DefaultPageSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/foo"+tt.query, nil)
			p := ParsePagination(r)
			if p.Page != tt.wantPage || p.PageSize != tt.wantSize {
				t.Errorf("got page=%d size=%d, want page=%d size=%d",
					p.Page, p.PageSize, tt.wantPage, tt.wantSize)
			}
		})
	}
}

func TestPagination_Offset(t *testing.T) {
	tests := []struct {
		page, size, offset int
	}{
		{1, 50, 0},
		{2, 50, 50},
		{5, 10, 40},
	}
	for _, tt := range tests {
		p := Pagination{Page: tt.page, PageSize: tt.size}
		if got := p.Offset(); got != tt.offset {
			t.Errorf("Pagination{%d,%d}.Offset() = %d, want %d",
				tt.page, tt.size, got, tt.offset)
		}
	}
}

func TestParseSort(t *testing.T) {
	tests := []struct {
		name  string
		query string
		def   string
		want  string
	}{
		{"empty falls back to default", "", "desc", "desc"},
		{"asc", "?sort=asc", "desc", "asc"},
		{"desc", "?sort=desc", "asc", "desc"},
		{"unknown falls back", "?sort=bogus", "asc", "asc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/foo"+tt.query, nil)
			if got := ParseSort(r, tt.def); got != tt.want {
				t.Errorf("ParseSort(%q, %q) = %q, want %q",
					tt.query, tt.def, got, tt.want)
			}
		})
	}
}
