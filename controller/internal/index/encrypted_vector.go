package index

import (
	"crypto/sha256"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"

	"github.com/user/extorvm/controller/internal/fhe"
	"github.com/user/extorvm/controller/internal/vectorindex"
)

// EncryptedVectorReplica is a shareable, FHE-encrypted representation
// of a local vector index. Peers can download it and perform similarity
// queries using homomorphic evaluation without learning document contents.
type EncryptedVectorReplica struct {
	Version   uint32
	Params    bgv.Parameters
	PublicKey *rlwe.PublicKey
	EvalKey   rlwe.EvaluationKeySet // for peers to evaluate dot products

	// Encrypted document vectors (one ciphertext per document).
	// Each ciphertext contains the quantized embedding vector packed into SIMD slots.
	DocVectors []*rlwe.Ciphertext

	// Plaintext metadata for filtering (not encrypted — intentional tradeoff).
	DocInfos []ReplicaDocInfo

	DocCount  int
	Dimension int
	Scale     float64  // quantization scale used
	Checksum  [32]byte // SHA-256 of serialized vector ciphertexts
	CreatedAt int64
}

// BuildEncryptedVectorReplica encrypts all vectors in the VectorIndex and
// attaches plaintext metadata from the InvertedIndex for filtering.
func BuildEncryptedVectorReplica(
	vi *vectorindex.VectorIndex,
	idx *InvertedIndex,
	keys *fhe.KeySet,
	scale float64,
) (*EncryptedVectorReplica, error) {
	if scale == 0 {
		scale = fhe.DefaultScale
	}

	params := keys.Params
	hasher := sha256.New()

	idx.mu.RLock()
	docCount := len(idx.Documents)

	var docVectors []*rlwe.Ciphertext
	var docInfos []ReplicaDocInfo

	// Process each document in order of ID.
	for docID := uint32(0); docID < uint32(docCount); docID++ {
		doc, ok := idx.Documents[docID]
		if !ok {
			continue
		}

		// Get the embedding vector from the VectorIndex.
		// We re-embed the document content to get the vector.
		// In a production implementation, VectorIndex would expose stored vectors.
		embedding, err := vi.GetEmbedding(int(docID))
		if err != nil || embedding == nil {
			// Document not in vector index; skip.
			continue
		}

		// Quantize the float32 vector to uint64.
		quantized := fhe.QuantizeVector(embedding, scale)

		// Encrypt the quantized vector.
		ct, err := fhe.EncryptVector(quantized, params, keys.PublicKey)
		if err != nil {
			idx.mu.RUnlock()
			return nil, err
		}

		// Add to checksum.
		ctBytes, _ := ct.MarshalBinary()
		hasher.Write(ctBytes)

		docVectors = append(docVectors, ct)
		docInfos = append(docInfos, DocMetaToReplicaInfo(doc))
	}
	idx.mu.RUnlock()

	var checksum [32]byte
	copy(checksum[:], hasher.Sum(nil))

	return &EncryptedVectorReplica{
		Version:    1,
		Params:     params,
		PublicKey:  keys.PublicKey,
		EvalKey:    keys.EvaluationKey,
		DocVectors: docVectors,
		DocInfos:   docInfos,
		DocCount:   len(docVectors),
		Dimension:  vi.Dimension(),
		Scale:      scale,
		Checksum:   checksum,
		CreatedAt:  time.Now().Unix(),
	}, nil
}

// SearchReplica performs an encrypted vector similarity search on the replica.
// The searcher encrypts their query with the owner's public key, evaluates
// dot products homomorphically, and the owner decrypts the scores.
//
// filter is applied to DocInfos BEFORE homomorphic evaluation to reduce computation.
func SearchReplica(
	replica *EncryptedVectorReplica,
	encryptedQuery *rlwe.Ciphertext,
	filter *MetadataFilter,
	keys *fhe.KeySet, // owner's keys for decryption
	k int,
) ([]SearchResult, error) {
	type scored struct {
		index int
		score float32
	}
	var matches []scored

	for i, ct := range replica.DocVectors {
		// Apply metadata filter before expensive homomorphic evaluation.
		if filter != nil && i < len(replica.DocInfos) && !filter.MatchesReplicaInfo(replica.DocInfos[i]) {
			continue
		}

		// Homomorphic dot product: encQuery * encDoc (element-wise).
		product, err := fhe.EvaluateElementWiseProduct(encryptedQuery, ct, replica.Params, replica.EvalKey)
		if err != nil {
			continue
		}

		// Owner decrypts the dot product score.
		score, err := fhe.DecryptDotProduct(product, keys.Params, keys.SecretKey, replica.Scale, replica.Dimension)
		if err != nil {
			continue
		}

		matches = append(matches, scored{index: i, score: score})
	}

	// Sort by score descending.
	for i := range matches {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].score > matches[i].score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	if k > 0 && len(matches) > k {
		matches = matches[:k]
	}

	var results []SearchResult
	for _, m := range matches {
		r := SearchResult{
			DocID: uint32(m.index),
			Score: m.score,
			Mode:  ModeVector,
		}
		if m.index < len(replica.DocInfos) {
			info := replica.DocInfos[m.index]
			r.FileType = info.FileType
			r.Size = info.Size
			r.CreatedAt = info.CreatedAt
			r.ModifiedAt = info.ModifiedAt
			r.WordCount = info.WordCount
		}
		results = append(results, r)
	}

	return results, nil
}
