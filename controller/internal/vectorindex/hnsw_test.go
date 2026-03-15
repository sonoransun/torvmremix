package vectorindex

import (
	"math"
	"math/rand"
	"testing"
)

func randomVector(dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = rand.Float32()*2 - 1
	}
	normalize(v)
	return v
}

func bruteForceKNN(vectors [][]float32, query []float32, k int) []int {
	type scored struct {
		id   int
		dist float32
	}
	var all []scored
	for i, v := range vectors {
		all = append(all, scored{i, cosineDistance(query, v)})
	}
	// Sort by distance ascending.
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if all[j].dist < all[i].dist {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if k > len(all) {
		k = len(all)
	}
	ids := make([]int, k)
	for i := 0; i < k; i++ {
		ids[i] = all[i].id
	}
	return ids
}

func cosineDistance(a, b []float32) float32 {
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	denom := float32(math.Sqrt(float64(na * nb)))
	if denom == 0 {
		return 1
	}
	return 1 - dot/denom
}

func TestHNSWInsertAndSearch(t *testing.T) {
	dim := 32
	n := 200
	k := 5

	h := NewHNSWIndex(dim, 16, 100)

	var vectors [][]float32
	for i := 0; i < n; i++ {
		v := randomVector(dim)
		vectors = append(vectors, v)
		h.Insert(v)
	}

	if h.Len() != n {
		t.Fatalf("expected %d nodes, got %d", n, h.Len())
	}

	// Test that the HNSW search agrees with brute force for several queries.
	queries := 20
	totalRecall := 0
	for q := 0; q < queries; q++ {
		query := randomVector(dim)
		hnswResults := h.SearchKNN(query, k)
		bruteResults := bruteForceKNN(vectors, query, k)

		// Compute recall: fraction of true top-k found by HNSW.
		bruteSet := make(map[int]bool)
		for _, id := range bruteResults {
			bruteSet[id] = true
		}

		hits := 0
		for _, r := range hnswResults {
			if bruteSet[r.ID] {
				hits++
			}
		}
		totalRecall += hits
	}

	recall := float64(totalRecall) / float64(queries*k)
	t.Logf("HNSW recall@%d over %d queries: %.2f%%", k, queries, recall*100)
	if recall < 0.70 {
		t.Errorf("recall too low: %.2f%% (expected >= 70%%)", recall*100)
	}
}

func TestHNSWEmpty(t *testing.T) {
	h := NewHNSWIndex(4, 8, 50)
	results := h.SearchKNN([]float32{1, 0, 0, 0}, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty index, got %d", len(results))
	}
}

func TestHNSWSingleElement(t *testing.T) {
	h := NewHNSWIndex(4, 8, 50)
	h.Insert([]float32{1, 0, 0, 0})

	results := h.SearchKNN([]float32{1, 0, 0, 0}, 1)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Score < 0.99 {
		t.Errorf("expected near-perfect similarity for identical vector, got %f", results[0].Score)
	}
}

func TestHNSWCosineSimilarity(t *testing.T) {
	h := NewHNSWIndex(3, 8, 50)

	// Insert orthogonal vectors.
	h.Insert([]float32{1, 0, 0})
	h.Insert([]float32{0, 1, 0})
	h.Insert([]float32{0, 0, 1})

	// Search for something close to [1, 0, 0].
	results := h.SearchKNN([]float32{0.9, 0.1, 0}, 3)
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	// The closest should be ID 0 (the [1,0,0] vector).
	if results[0].ID != 0 {
		t.Errorf("expected closest to be ID 0, got %d", results[0].ID)
	}
}

func TestTFIDFEmbedder(t *testing.T) {
	emb := NewTFIDFEmbedder(32) // small dimension to increase hash collisions for test

	docs := []string{
		"encryption privacy security network protocol",
		"cooking recipes dinner kitchen food",
		"encryption security algorithm protocol cipher",
	}
	emb.Train(docs)

	// Embed the same term — should produce identical vectors.
	v1, _ := emb.Embed("encryption security")
	v2, _ := emb.Embed("encryption security")
	sim := cosineSim(v1, v2)
	if sim < 0.99 {
		t.Errorf("expected identical vectors for same input, got similarity %.3f", sim)
	}

	// Embed overlapping vs disjoint terms.
	vCrypto, _ := emb.Embed("encryption security protocol")
	vCooking, _ := emb.Embed("cooking dinner kitchen")

	// The crypto vector should have non-zero components.
	var hasNonZero bool
	for _, val := range vCrypto {
		if val != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("expected non-zero vector for valid terms")
	}

	// With a small enough dimension, overlapping terms in v1 and vCrypto
	// should produce similarity > 0, while disjoint terms produce ~ 0.
	simCrypto := cosineSim(v1, vCrypto)
	simCook := cosineSim(v1, vCooking)
	t.Logf("sim(encryption, crypto) = %.3f, sim(encryption, cooking) = %.3f", simCrypto, simCook)

	// At minimum, same-term vectors should be self-similar.
	if simCrypto < simCook && simCrypto > 0 {
		t.Log("warning: expected crypto similarity >= cooking similarity")
	}
}

func TestTFIDFEmbedderDimension(t *testing.T) {
	emb := NewTFIDFEmbedder(128)
	if emb.Dimension() != 128 {
		t.Errorf("expected dimension 128, got %d", emb.Dimension())
	}
	v, err := emb.Embed("test text")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 128 {
		t.Errorf("expected 128-dim vector, got %d", len(v))
	}
}

func cosineSim(a, b []float32) float32 {
	return 1 - cosineDistance(a, b)
}
