package vectorindex

import (
	"math"
	"strings"
	"unicode"
)

// TFIDFEmbedder computes TF-IDF sparse-ish vectors in pure Go.
// It hashes terms into a fixed-dimension vector space and weights
// by term frequency × inverse document frequency.
type TFIDFEmbedder struct {
	dimension int
	docFreq   map[string]int // term → number of documents containing it
	docCount  int
	trained   bool
}

// NewTFIDFEmbedder creates a TF-IDF embedder with the given output dimension.
func NewTFIDFEmbedder(dimension int) *TFIDFEmbedder {
	return &TFIDFEmbedder{
		dimension: dimension,
		docFreq:   make(map[string]int),
	}
}

// Train builds the IDF statistics from a corpus of documents.
// Each string in documents is the full text of one document.
func (e *TFIDFEmbedder) Train(documents []string) {
	e.docFreq = make(map[string]int)
	e.docCount = len(documents)

	for _, doc := range documents {
		seen := make(map[string]bool)
		for _, term := range tokenizeSimple(doc) {
			if !seen[term] {
				seen[term] = true
				e.docFreq[term]++
			}
		}
	}
	e.trained = true
}

// Embed produces a fixed-dimension vector from text using TF-IDF weights.
// Terms are hashed into dimension buckets; collisions are additive.
func (e *TFIDFEmbedder) Embed(text string) ([]float32, error) {
	vec := make([]float32, e.dimension)
	terms := tokenizeSimple(text)
	if len(terms) == 0 {
		return vec, nil
	}

	// Count term frequencies.
	tf := make(map[string]int)
	for _, t := range terms {
		tf[t]++
	}

	// Compute TF-IDF weighted vector.
	for term, count := range tf {
		// Term frequency: log(1 + count).
		tfWeight := float32(math.Log(1 + float64(count)))

		// Inverse document frequency: log(1 + N / (1 + df)).
		// The +1 inside the log ensures terms in all documents still
		// have non-zero weight (standard smoothed IDF).
		df := e.docFreq[term]
		n := e.docCount
		if n == 0 {
			n = 1
		}
		idfWeight := float32(math.Log(1 + float64(n)/float64(1+df)))

		// Hash term to a dimension bucket.
		bucket := hashStringToBucket(term, e.dimension)
		vec[bucket] += tfWeight * idfWeight
	}

	// L2 normalize the vector.
	normalize(vec)

	return vec, nil
}

// Dimension returns the output vector dimensionality.
func (e *TFIDFEmbedder) Dimension() int { return e.dimension }

// Close is a no-op for the TF-IDF embedder.
func (e *TFIDFEmbedder) Close() error { return nil }

// tokenizeSimple splits text into lowercase alphanumeric tokens.
func tokenizeSimple(text string) []string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var tokens []string
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) >= 2 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// hashStringToBucket uses FNV-1a to hash a string into [0, buckets).
func hashStringToBucket(s string, buckets int) int {
	h := uint32(2166136261) // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619 // FNV prime
	}
	return int(h % uint32(buckets))
}

// normalize L2-normalizes a vector in place.
func normalize(v []float32) {
	var sum float32
	for _, val := range v {
		sum += val * val
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(float64(sum)))
	for i := range v {
		v[i] /= norm
	}
}
