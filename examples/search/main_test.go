package main

import (
	"encoding/json"
	"slices"
	"testing"

	alfred "github.com/brookke/alfred-go"
)

func args(t *testing.T, out alfred.Output) []string {
	t.Helper()
	b, err := json.Marshal(out.Response())
	if err != nil {
		t.Fatal(err)
	}
	var response struct{ Items []struct{ Arg string } }
	if err := json.Unmarshal(b, &response); err != nil {
		t.Fatal(err)
	}
	var result []string
	for _, item := range response.Items {
		result = append(result, item.Arg)
	}
	return result
}

func TestSearchEscapesTheQuery(t *testing.T) {
	want := []string{
		"https://www.google.com/search?q=go+%26+alfred",
		"https://duckduckgo.com/?q=go+%26+alfred",
		"https://en.wikipedia.org/w/index.php?search=go+%26+alfred",
	}
	if got := args(t, search(" go & alfred ")); !slices.Equal(got, want) {
		t.Errorf("search = %v, want %v", got, want)
	}
}

func TestEmptyQueryShowsHint(t *testing.T) {
	if got := args(t, search("  ")); !slices.Equal(got, []string{""}) {
		t.Errorf("empty query should show one hint, got %v", got)
	}
}
