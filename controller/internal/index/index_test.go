package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/extorvm/controller/internal/fhe"
)

func TestTokenize(t *testing.T) {
	terms := Tokenize("The quick brown fox jumps over the lazy dog")
	// "the" is a stop word, "fox" is 3 chars (included)
	expected := map[string]bool{
		"quick": true, "brown": true, "fox": true,
		"jumps": true, "over": true, "lazy": true, "dog": true,
	}
	for _, term := range terms {
		if !expected[term] {
			t.Errorf("unexpected term: %q", term)
		}
		delete(expected, term)
	}
	for term := range expected {
		t.Errorf("missing term: %q", term)
	}
}

func TestTokenizeDedup(t *testing.T) {
	terms := Tokenize("hello hello hello world world")
	seen := make(map[string]int)
	for _, term := range terms {
		seen[term]++
	}
	for term, count := range seen {
		if count > 1 {
			t.Errorf("term %q appeared %d times (should be deduplicated)", term, count)
		}
	}
}

func TestTokenizeStopWords(t *testing.T) {
	terms := Tokenize("the a an is are was were")
	if len(terms) != 0 {
		t.Errorf("expected no terms from stop words, got %v", terms)
	}
}

func TestInvertedIndexBasic(t *testing.T) {
	idx := NewInvertedIndex()

	idx.AddDocument("/tmp/doc1.txt", []byte("encryption and privacy"))
	idx.AddDocument("/tmp/doc2.txt", []byte("privacy and security"))
	idx.AddDocument("/tmp/doc3.txt", []byte("performance tuning"))

	// "privacy" appears in doc1 and doc2.
	results := idx.Query("privacy")
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'privacy', got %d", len(results))
	}

	// "tuning" appears in doc3 only.
	results = idx.Query("tuning")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'tuning', got %d", len(results))
	}

	// "notfound" should return nothing.
	results = idx.Query("notfound")
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'notfound', got %d", len(results))
	}

	if idx.TermCount() == 0 {
		t.Error("expected non-zero term count")
	}
	if idx.DocCount() != 3 {
		t.Errorf("expected 3 docs, got %d", idx.DocCount())
	}
}

func TestIndexDirectory(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "doc1.txt"), []byte("hello world encryption"), 0644)
	os.WriteFile(filepath.Join(dir, "doc2.txt"), []byte("privacy security"), 0644)
	os.WriteFile(filepath.Join(dir, "image.png"), []byte{0x89, 0x50}, 0644) // not a supported extension

	idx := NewInvertedIndex()
	count, err := IndexDirectory(idx, dir)
	if err != nil {
		t.Fatalf("IndexDirectory: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 indexed docs, got %d", count)
	}
	if idx.DocCount() != 2 {
		t.Errorf("expected 2 docs in index, got %d", idx.DocCount())
	}
}

func TestEncryptedIndexBuildAndQuery(t *testing.T) {
	params, err := fhe.NewParams(fhe.DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	keys, err := fhe.GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	idx := NewInvertedIndex()
	idx.AddDocument("/tmp/doc1.txt", []byte("homomorphic encryption search"))
	idx.AddDocument("/tmp/doc2.txt", []byte("privacy preserving queries"))

	encIdx, err := BuildEncryptedIndex(idx, keys)
	if err != nil {
		t.Fatalf("BuildEncryptedIndex: %v", err)
	}

	if encIdx.PageCount == 0 {
		t.Fatal("expected at least one term page")
	}
	if encIdx.TermCount == 0 {
		t.Fatal("expected non-zero term count")
	}

	// Search for a term we know is in the index.
	encQuery, err := EncryptedQuery("encryption", encIdx)
	if err != nil {
		t.Fatalf("EncryptedQuery: %v", err)
	}

	results, err := DelegatedEvaluate(encQuery, encIdx)
	if err != nil {
		t.Fatalf("DelegatedEvaluate: %v", err)
	}

	matched, err := DecryptResults(results, encIdx, keys)
	if err != nil {
		t.Fatalf("DecryptResults: %v", err)
	}

	if len(matched) == 0 {
		t.Error("expected at least one match for 'encryption'")
	}

	// Search for a term NOT in the index.
	encQuery2, err := EncryptedQuery("blockchain", encIdx)
	if err != nil {
		t.Fatalf("EncryptedQuery: %v", err)
	}
	results2, err := DelegatedEvaluate(encQuery2, encIdx)
	if err != nil {
		t.Fatalf("DelegatedEvaluate: %v", err)
	}
	matched2, err := DecryptResults(results2, encIdx, keys)
	if err != nil {
		t.Fatalf("DecryptResults: %v", err)
	}
	if len(matched2) != 0 {
		t.Errorf("expected 0 matches for 'blockchain', got %d", len(matched2))
	}
}
