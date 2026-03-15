package index

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/user/extorvm/controller/internal/fhe"
)

// DocMeta holds metadata about an indexed document.
type DocMeta struct {
	ID         uint32
	Path       string
	Title      string
	Size       int64
	IndexedAt  time.Time
	FileType   string    // "text", "markdown", "json", "xml", "html", "config", "log", "data"
	MimeType   string    // "text/plain", "application/json", etc.
	CreatedAt  time.Time // file creation/change time from OS
	ModifiedAt time.Time // file modification time
	Tags       []string  // user-defined tags
	WordCount  int       // approximate word count
}

// InvertedIndex is a plaintext inverted index mapping term hashes to document IDs.
type InvertedIndex struct {
	mu        sync.RWMutex
	Terms     map[uint64][]uint32 // term_hash → doc IDs
	Documents map[uint32]DocMeta
	nextDocID uint32
}

// NewInvertedIndex creates an empty inverted index.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		Terms:     make(map[uint64][]uint32),
		Documents: make(map[uint32]DocMeta),
	}
}

// AddDocument tokenizes and indexes a document's content.
func (idx *InvertedIndex) AddDocument(path string, content []byte) uint32 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docID := idx.nextDocID
	idx.nextDocID++

	fi, _ := os.Stat(path)
	var size int64
	var modTime, createTime time.Time
	if fi != nil {
		size = fi.Size()
		modTime = fi.ModTime()
		createTime = fi.ModTime() // creation time not portable; use mtime as fallback
	}

	ext := filepath.Ext(path)
	wordCount := len(Tokenize(string(content)))

	idx.Documents[docID] = DocMeta{
		ID:         docID,
		Path:       path,
		Title:      filepath.Base(path),
		Size:       size,
		IndexedAt:  time.Now(),
		FileType:   DetectFileType(ext),
		MimeType:   DetectMimeType(ext),
		CreatedAt:  createTime,
		ModifiedAt: modTime,
		WordCount:  wordCount,
	}

	terms := Tokenize(string(content))
	for _, term := range terms {
		h := fhe.HashTerm(term)
		idx.Terms[h] = appendUnique(idx.Terms[h], docID)
	}

	return docID
}

// Query returns document IDs matching a search term.
func (idx *InvertedIndex) Query(term string) []uint32 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	h := fhe.HashTerm(term)
	return idx.Terms[h]
}

// TermCount returns the number of unique terms in the index.
func (idx *InvertedIndex) TermCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.Terms)
}

// DocCount returns the number of indexed documents.
func (idx *InvertedIndex) DocCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.Documents)
}

// GetDoc returns metadata for a document by ID.
func (idx *InvertedIndex) GetDoc(id uint32) (DocMeta, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	doc, ok := idx.Documents[id]
	return doc, ok
}

// TermHashes returns all unique term hashes in the index.
func (idx *InvertedIndex) TermHashes() []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	hashes := make([]uint64, 0, len(idx.Terms))
	for h := range idx.Terms {
		hashes = append(hashes, h)
	}
	return hashes
}

func appendUnique(slice []uint32, val uint32) []uint32 {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
