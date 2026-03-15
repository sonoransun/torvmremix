package index

import (
	"testing"
	"time"
)

func TestMetadataFilterMatchesAll(t *testing.T) {
	doc := DocMeta{
		FileType:   "text",
		Size:       1024,
		CreatedAt:  time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		ModifiedAt: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	// Nil filter matches everything.
	var f *MetadataFilter
	if !f.Matches(doc) {
		t.Error("nil filter should match")
	}

	// Empty filter matches everything.
	f = &MetadataFilter{}
	if !f.Matches(doc) {
		t.Error("empty filter should match")
	}
}

func TestMetadataFilterFileType(t *testing.T) {
	doc := DocMeta{FileType: "json", Size: 100}

	f := &MetadataFilter{FileTypes: []string{"json", "xml"}}
	if !f.Matches(doc) {
		t.Error("expected json doc to match json/xml filter")
	}

	f = &MetadataFilter{FileTypes: []string{"markdown"}}
	if f.Matches(doc) {
		t.Error("expected json doc NOT to match markdown filter")
	}
}

func TestMetadataFilterSizeRange(t *testing.T) {
	doc := DocMeta{Size: 5000}

	f := &MetadataFilter{MinSize: 1000, MaxSize: 10000}
	if !f.Matches(doc) {
		t.Error("5000 should be within 1000-10000")
	}

	f = &MetadataFilter{MinSize: 10000}
	if f.Matches(doc) {
		t.Error("5000 should fail min=10000")
	}

	f = &MetadataFilter{MaxSize: 1000}
	if f.Matches(doc) {
		t.Error("5000 should fail max=1000")
	}
}

func TestMetadataFilterDateRange(t *testing.T) {
	doc := DocMeta{
		ModifiedAt: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	f := &MetadataFilter{
		ModifiedAfter: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	if !f.Matches(doc) {
		t.Error("june 15 should be after june 1")
	}

	f = &MetadataFilter{
		ModifiedAfter: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
	}
	if f.Matches(doc) {
		t.Error("june 15 should NOT be after july 1")
	}
}

func TestMetadataFilterTags(t *testing.T) {
	doc := DocMeta{Tags: []string{"important", "crypto"}}

	f := &MetadataFilter{Tags: []string{"crypto"}}
	if !f.Matches(doc) {
		t.Error("doc with crypto tag should match")
	}

	f = &MetadataFilter{Tags: []string{"crypto", "important"}}
	if !f.Matches(doc) {
		t.Error("doc with both tags should match AND filter")
	}

	f = &MetadataFilter{Tags: []string{"missing"}}
	if f.Matches(doc) {
		t.Error("doc without missing tag should not match")
	}
}

func TestDetectFileType(t *testing.T) {
	tests := map[string]string{
		".txt":  "text",
		".md":   "markdown",
		".json": "json",
		".xml":  "xml",
		".html": "html",
		".yaml": "yaml",
		".cfg":  "config",
		".log":  "log",
		".csv":  "data",
		".xyz":  "unknown",
	}
	for ext, expected := range tests {
		got := DetectFileType(ext)
		if got != expected {
			t.Errorf("DetectFileType(%q) = %q, want %q", ext, got, expected)
		}
	}
}

func TestReplicaDocInfoFilter(t *testing.T) {
	info := ReplicaDocInfo{
		FileType:   "json",
		Size:       2048,
		CreatedAt:  time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC).Unix(),
		ModifiedAt: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC).Unix(),
	}

	f := &MetadataFilter{FileTypes: []string{"json"}, MinSize: 1000}
	if !f.MatchesReplicaInfo(info) {
		t.Error("json + size 2048 should match json filter with min 1000")
	}

	f = &MetadataFilter{FileTypes: []string{"xml"}}
	if f.MatchesReplicaInfo(info) {
		t.Error("json should not match xml filter")
	}
}
