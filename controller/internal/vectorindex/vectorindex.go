package vectorindex

import (
	"sync"
)

// VectorIndex coordinates document embedding and HNSW indexing
// for semantic similarity search.
type VectorIndex struct {
	mu       sync.RWMutex
	embedder Embedder
	hnsw     *HNSWIndex
	docs     map[int]DocRef // internal HNSW ID → document reference
	ready    bool
}

// DocRef is a reference to an indexed document.
type DocRef struct {
	DocID uint32 // shared with keyword index
	Path  string
	Title string
}

// NewVectorIndex creates a vector index with the given embedder and HNSW parameters.
func NewVectorIndex(embedder Embedder, m, efConstruct int) *VectorIndex {
	return &VectorIndex{
		embedder: embedder,
		hnsw:     NewHNSWIndex(embedder.Dimension(), m, efConstruct),
		docs:     make(map[int]DocRef),
	}
}

// AddDocument embeds the document content and inserts it into the HNSW index.
func (vi *VectorIndex) AddDocument(docID uint32, path, title string, content []byte) error {
	vec, err := vi.embedder.Embed(string(content))
	if err != nil {
		return err
	}

	vi.mu.Lock()
	hnswID := vi.hnsw.Insert(vec)
	vi.docs[hnswID] = DocRef{
		DocID: docID,
		Path:  path,
		Title: title,
	}
	vi.ready = true
	vi.mu.Unlock()

	return nil
}

// Search finds the k most similar documents to the query text.
func (vi *VectorIndex) Search(query string, k int) ([]SearchHit, error) {
	vi.mu.RLock()
	defer vi.mu.RUnlock()

	if !vi.ready || vi.hnsw.Len() == 0 {
		return nil, nil
	}

	queryVec, err := vi.embedder.Embed(query)
	if err != nil {
		return nil, err
	}

	hits := vi.hnsw.SearchKNN(queryVec, k)

	// Map HNSW IDs back to document references.
	for i := range hits {
		if ref, ok := vi.docs[hits[i].ID]; ok {
			hits[i].ID = int(ref.DocID)
		}
	}

	return hits, nil
}

// Len returns the number of indexed documents.
func (vi *VectorIndex) Len() int {
	vi.mu.RLock()
	defer vi.mu.RUnlock()
	return len(vi.docs)
}

// IsReady returns whether the index has been populated.
func (vi *VectorIndex) IsReady() bool {
	vi.mu.RLock()
	defer vi.mu.RUnlock()
	return vi.ready
}

// Dimension returns the embedding vector dimensionality.
func (vi *VectorIndex) Dimension() int {
	return vi.embedder.Dimension()
}

// GetEmbedding returns the stored embedding vector for a document by its
// shared docID. Returns nil if the document is not in the vector index.
func (vi *VectorIndex) GetEmbedding(docID int) ([]float32, error) {
	vi.mu.RLock()
	defer vi.mu.RUnlock()

	// Find the HNSW node ID for this docID.
	for hnswID, ref := range vi.docs {
		if int(ref.DocID) == docID {
			dim := vi.hnsw.dimension
			offset := hnswID * dim
			if offset+dim > len(vi.hnsw.vectors) {
				return nil, nil
			}
			vec := make([]float32, dim)
			copy(vec, vi.hnsw.vectors[offset:offset+dim])
			return vec, nil
		}
	}
	return nil, nil
}

// Close releases embedder resources.
func (vi *VectorIndex) Close() error {
	return vi.embedder.Close()
}
