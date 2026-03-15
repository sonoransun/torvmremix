// Package vectorindex provides HNSW approximate nearest neighbor search
// and embedding-based vector indexing for semantic document search.
package vectorindex

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
)

// SearchHit represents a single search result with distance score.
type SearchHit struct {
	ID    int
	Score float32 // cosine similarity (higher = more similar)
}

// hnswNode stores the graph connections for one vector in the HNSW index.
type hnswNode struct {
	connections [][]int // connections[level] = list of neighbor IDs
	level       int
}

// HNSWIndex implements the Hierarchical Navigable Small World graph
// for approximate nearest neighbor search using cosine similarity.
type HNSWIndex struct {
	mu          sync.RWMutex
	dimension   int
	m           int // max connections per layer
	mMax0       int // max connections at layer 0 (2*m)
	efConstruct int // beam width during construction
	efSearch    int // default beam width during search
	maxLevel    int
	entryPoint  int
	nodes       []hnswNode
	vectors     []float32 // flat array: node i at [i*dimension : (i+1)*dimension]
	mL          float64   // normalization factor for level generation
}

// NewHNSWIndex creates an empty HNSW index with the given parameters.
func NewHNSWIndex(dimension, m, efConstruct int) *HNSWIndex {
	return &HNSWIndex{
		dimension:   dimension,
		m:           m,
		mMax0:       2 * m,
		efConstruct: efConstruct,
		efSearch:    50,
		entryPoint:  -1,
		mL:          1.0 / math.Log(float64(m)),
	}
}

// Len returns the number of vectors in the index.
func (h *HNSWIndex) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// Dimension returns the vector dimensionality.
func (h *HNSWIndex) Dimension() int { return h.dimension }

// Insert adds a vector to the index. The ID is assigned sequentially.
func (h *HNSWIndex) Insert(vector []float32) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := len(h.nodes)
	level := h.randomLevel()

	node := hnswNode{
		connections: make([][]int, level+1),
		level:       level,
	}
	h.nodes = append(h.nodes, node)
	h.vectors = append(h.vectors, vector...)

	if h.entryPoint < 0 {
		h.entryPoint = id
		h.maxLevel = level
		return id
	}

	// Find entry point at the top level and greedily descend.
	ep := h.entryPoint
	for lc := h.maxLevel; lc > level; lc-- {
		ep = h.greedyClosest(vector, ep, lc)
	}

	// For each level from min(level, maxLevel) down to 0, find neighbors and connect.
	for lc := min(level, h.maxLevel); lc >= 0; lc-- {
		candidates := h.searchLayer(vector, ep, h.efConstruct, lc)

		maxConn := h.m
		if lc == 0 {
			maxConn = h.mMax0
		}

		neighbors := h.selectNeighborsSimple(candidates, maxConn)

		// Set connections for the new node.
		h.nodes[id].connections[lc] = neighbors

		// Add reverse connections.
		for _, n := range neighbors {
			h.nodes[n].connections[lc] = append(h.nodes[n].connections[lc], id)
			// Prune if over capacity using distance-aware selection.
			if len(h.nodes[n].connections[lc]) > maxConn {
				h.nodes[n].connections[lc] = h.selectNeighborsForNode(
					n, h.nodes[n].connections[lc], maxConn)
			}
		}

		if len(candidates) > 0 {
			ep = candidates[0]
		}
	}

	if level > h.maxLevel {
		h.maxLevel = level
		h.entryPoint = id
	}

	return id
}

// SearchKNN returns the k nearest neighbors to the query vector.
func (h *HNSWIndex) SearchKNN(query []float32, k int) []SearchHit {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.entryPoint < 0 || len(h.nodes) == 0 {
		return nil
	}

	ep := h.entryPoint
	for lc := h.maxLevel; lc > 0; lc-- {
		ep = h.greedyClosest(query, ep, lc)
	}

	ef := h.efSearch
	if ef < k {
		ef = k
	}

	candidates := h.searchLayer(query, ep, ef, 0)

	if len(candidates) > k {
		candidates = candidates[:k]
	}

	results := make([]SearchHit, len(candidates))
	for i, c := range candidates {
		results[i] = SearchHit{
			ID:    c,
			Score: h.cosineSimilarity(query, c),
		}
	}

	return results
}

// searchLayer performs a beam search at a single HNSW layer.
// Returns candidate IDs sorted by distance (closest first).
func (h *HNSWIndex) searchLayer(query []float32, entryID, ef, level int) []int {
	visited := make(map[int]bool)
	visited[entryID] = true

	candidates := &distHeap{query: query, h: h, isMaxHeap: false}
	results := &distHeap{query: query, h: h, isMaxHeap: true}

	heap.Push(candidates, entryID)
	heap.Push(results, entryID)

	for candidates.Len() > 0 {
		nearest := heap.Pop(candidates).(int)

		// If nearest candidate is farther than the farthest result, stop.
		if results.Len() >= ef {
			farthest := results.Peek()
			if h.distance(query, nearest) > h.distance(query, farthest) {
				break
			}
		}

		// Explore neighbors.
		if level < len(h.nodes[nearest].connections) {
			for _, neighbor := range h.nodes[nearest].connections[level] {
				if visited[neighbor] {
					continue
				}
				visited[neighbor] = true

				farthestResult := -1
				if results.Len() > 0 {
					farthestResult = results.Peek()
				}

				if results.Len() < ef || h.distance(query, neighbor) < h.distance(query, farthestResult) {
					heap.Push(candidates, neighbor)
					heap.Push(results, neighbor)
					if results.Len() > ef {
						heap.Pop(results)
					}
				}
			}
		}
	}

	// Extract results sorted by distance (closest first).
	out := make([]int, results.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(results).(int)
	}
	return out
}

// greedyClosest finds the closest node to query at the given level
// by greedily following edges.
func (h *HNSWIndex) greedyClosest(query []float32, ep, level int) int {
	cur := ep
	curDist := h.distance(query, cur)
	for {
		changed := false
		if level < len(h.nodes[cur].connections) {
			for _, neighbor := range h.nodes[cur].connections[level] {
				d := h.distance(query, neighbor)
				if d < curDist {
					cur = neighbor
					curDist = d
					changed = true
				}
			}
		}
		if !changed {
			return cur
		}
	}
}

// selectNeighborsForNode selects the m closest neighbors to the node's
// own vector from a candidate list.
func (h *HNSWIndex) selectNeighborsForNode(nodeID int, candidates []int, m int) []int {
	if len(candidates) <= m {
		out := make([]int, len(candidates))
		copy(out, candidates)
		return out
	}

	nodeVec := h.vectors[nodeID*h.dimension : (nodeID+1)*h.dimension]

	// Sort candidates by distance to node (closest first).
	type scored struct {
		id   int
		dist float32
	}
	sc := make([]scored, len(candidates))
	for i, c := range candidates {
		sc[i] = scored{c, h.distance(nodeVec, c)}
	}
	for i := range sc {
		for j := i + 1; j < len(sc); j++ {
			if sc[j].dist < sc[i].dist {
				sc[i], sc[j] = sc[j], sc[i]
			}
		}
	}

	out := make([]int, m)
	for i := 0; i < m; i++ {
		out[i] = sc[i].id
	}
	return out
}

// selectNeighborsSimple keeps the first m candidates (assumed pre-sorted by searchLayer).
func (h *HNSWIndex) selectNeighborsSimple(candidates []int, m int) []int {
	if len(candidates) <= m {
		out := make([]int, len(candidates))
		copy(out, candidates)
		return out
	}
	out := make([]int, m)
	copy(out, candidates[:m])
	return out
}

// distance returns the cosine distance (1 - cosine_similarity) between
// the query vector and the vector at the given node ID.
func (h *HNSWIndex) distance(query []float32, id int) float32 {
	return 1.0 - h.cosineSimilarity(query, id)
}

// cosineSimilarity computes the cosine similarity between the query
// and the vector stored at the given node ID.
func (h *HNSWIndex) cosineSimilarity(query []float32, id int) float32 {
	offset := id * h.dimension
	vec := h.vectors[offset : offset+h.dimension]

	var dot, normA, normB float32
	for i := 0; i < h.dimension; i++ {
		dot += query[i] * vec[i]
		normA += query[i] * query[i]
		normB += vec[i] * vec[i]
	}

	denom := float32(math.Sqrt(float64(normA * normB)))
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// randomLevel generates a random level for a new node using the
// exponential distribution: floor(-ln(uniform(0,1)) * mL).
func (h *HNSWIndex) randomLevel() int {
	return int(math.Floor(-math.Log(rand.Float64()) * h.mL))
}

// distHeap implements heap.Interface for node IDs ordered by distance.
type distHeap struct {
	ids       []int
	query     []float32
	h         *HNSWIndex
	isMaxHeap bool // true = max-heap (farthest first), false = min-heap (closest first)
}

func (dh *distHeap) Len() int { return len(dh.ids) }
func (dh *distHeap) Less(i, j int) bool {
	di := dh.h.distance(dh.query, dh.ids[i])
	dj := dh.h.distance(dh.query, dh.ids[j])
	if dh.isMaxHeap {
		return di > dj
	}
	return di < dj
}
func (dh *distHeap) Swap(i, j int) { dh.ids[i], dh.ids[j] = dh.ids[j], dh.ids[i] }
func (dh *distHeap) Push(x interface{}) {
	dh.ids = append(dh.ids, x.(int))
}
func (dh *distHeap) Pop() interface{} {
	old := dh.ids
	n := len(old)
	x := old[n-1]
	dh.ids = old[:n-1]
	return x
}
func (dh *distHeap) Peek() int {
	if len(dh.ids) == 0 {
		return -1
	}
	return dh.ids[0]
}
