package ingestion

import (
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("", "", "x", "y") != "x" {
		t.Fatal("should return first non-empty")
	}
	if firstNonEmpty("", "") != "" {
		t.Fatal("all empty → empty")
	}
}

func TestPtrIf(t *testing.T) {
	if ptrIf("") != nil {
		t.Fatal("empty → nil")
	}
	if v := ptrIf("a"); v == nil || *v != "a" {
		t.Fatal("non-empty → ptr")
	}
}

func TestAuthor(t *testing.T) {
	if author(&gofeed.Item{}) != "" {
		t.Fatal("no author → empty")
	}
	if author(&gofeed.Item{Authors: []*gofeed.Person{{Name: "Jane"}}}) != "Jane" {
		t.Fatal("authors[0]")
	}
	if author(&gofeed.Item{Author: &gofeed.Person{Name: "Bob"}}) != "Bob" {
		t.Fatal("legacy author")
	}
}
