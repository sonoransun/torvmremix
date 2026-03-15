package index

import (
	"path/filepath"
	"sort"

	"github.com/user/extorvm/controller/internal/vectorindex"
)

// SearchMode determines which index backends to use for a search.
type SearchMode string

const (
	ModeKeyword SearchMode = "keyword"
	ModeVector  SearchMode = "vector"
	ModeHybrid  SearchMode = "hybrid"
)

// SearchResult represents a single search hit from any mode.
type SearchResult struct {
	DocID      uint32
	Path       string
	Title      string
	Score      float32
	Mode       SearchMode // which index produced this result
	FileType   string     // from DocMeta
	Size       int64
	CreatedAt  int64      // Unix timestamp
	ModifiedAt int64
	WordCount  int
}

// CompositeIndex coordinates keyword (inverted) and vector (HNSW) indexes.
type CompositeIndex struct {
	Keyword *InvertedIndex
	Vector  *vectorindex.VectorIndex // nil if vector search disabled
}

// NewCompositeIndex creates a composite index. If vi is nil, only keyword
// search is available.
func NewCompositeIndex(vi *vectorindex.VectorIndex) *CompositeIndex {
	return &CompositeIndex{
		Keyword: NewInvertedIndex(),
		Vector:  vi,
	}
}

// AddDocument indexes a document in both keyword and vector indexes.
func (ci *CompositeIndex) AddDocument(path string, content []byte) uint32 {
	docID := ci.Keyword.AddDocument(path, content)

	if ci.Vector != nil {
		title := filepath.Base(path)
		ci.Vector.AddDocument(docID, path, title, content)
	}

	return docID
}

// Search dispatches to the appropriate index backend(s) based on mode.
// If filter is non-nil, results are filtered by document metadata.
func (ci *CompositeIndex) Search(query string, mode SearchMode, k int, filter *MetadataFilter) []SearchResult {
	var results []SearchResult
	switch mode {
	case ModeKeyword:
		results = ci.keywordSearch(query, k*2) // fetch extra before filtering
	case ModeVector:
		results = ci.vectorSearch(query, k*2)
	case ModeHybrid:
		results = ci.hybridSearch(query, k*2)
	default:
		results = ci.keywordSearch(query, k*2)
	}

	if filter != nil && !filter.IsEmpty() {
		var filtered []SearchResult
		for _, r := range results {
			doc, ok := ci.Keyword.GetDoc(r.DocID)
			if ok && filter.Matches(doc) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	if k > 0 && len(results) > k {
		results = results[:k]
	}
	return results
}

func (ci *CompositeIndex) keywordSearch(query string, k int) []SearchResult {
	docs := LocalQuery(ci.Keyword, query)
	var results []SearchResult
	for i, doc := range docs {
		if k > 0 && i >= k {
			break
		}
		results = append(results, SearchResult{
			DocID:      doc.ID,
			Path:       doc.Path,
			Title:      doc.Title,
			Score:      1.0,
			Mode:       ModeKeyword,
			FileType:   doc.FileType,
			Size:       doc.Size,
			CreatedAt:  doc.CreatedAt.Unix(),
			ModifiedAt: doc.ModifiedAt.Unix(),
			WordCount:  doc.WordCount,
		})
	}
	return results
}

func (ci *CompositeIndex) vectorSearch(query string, k int) []SearchResult {
	if ci.Vector == nil {
		return nil
	}
	hits, err := ci.Vector.Search(query, k)
	if err != nil || len(hits) == 0 {
		return nil
	}

	var results []SearchResult
	for _, hit := range hits {
		doc, ok := ci.Keyword.GetDoc(uint32(hit.ID))
		r := SearchResult{
			DocID: uint32(hit.ID),
			Score: hit.Score,
			Mode:  ModeVector,
		}
		if ok {
			r.Path = doc.Path
			r.Title = doc.Title
			r.FileType = doc.FileType
			r.Size = doc.Size
			r.CreatedAt = doc.CreatedAt.Unix()
			r.ModifiedAt = doc.ModifiedAt.Unix()
			r.WordCount = doc.WordCount
		}
		results = append(results, r)
	}
	return results
}

// hybridSearch combines keyword and vector results using Reciprocal Rank Fusion.
func (ci *CompositeIndex) hybridSearch(query string, k int) []SearchResult {
	kwResults := ci.keywordSearch(query, k*2) // fetch more for better fusion
	vecResults := ci.vectorSearch(query, k*2)

	// RRF scoring: score(doc) = sum(1/(60 + rank))
	const rrfK = 60
	scores := make(map[uint32]float32)
	paths := make(map[uint32]string)
	titles := make(map[uint32]string)

	for rank, r := range kwResults {
		scores[r.DocID] += 1.0 / float32(rrfK+rank)
		paths[r.DocID] = r.Path
		titles[r.DocID] = r.Title
	}
	for rank, r := range vecResults {
		scores[r.DocID] += 1.0 / float32(rrfK+rank)
		if _, ok := paths[r.DocID]; !ok {
			paths[r.DocID] = r.Path
			titles[r.DocID] = r.Title
		}
	}

	// Sort by RRF score descending.
	type scored struct {
		docID uint32
		score float32
	}
	var ranked []scored
	for id, s := range scores {
		ranked = append(ranked, scored{id, s})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	var results []SearchResult
	for i, r := range ranked {
		if k > 0 && i >= k {
			break
		}
		results = append(results, SearchResult{
			DocID: r.docID,
			Path:  paths[r.docID],
			Title: titles[r.docID],
			Score: r.score,
			Mode:  ModeHybrid,
		})
	}
	return results
}
