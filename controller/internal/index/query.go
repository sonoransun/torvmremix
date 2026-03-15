package index

import (
	"github.com/tuneinsight/lattigo/v6/core/rlwe"

	"github.com/user/extorvm/controller/internal/fhe"
)

// LocalQuery searches the plaintext index and returns matching document metadata.
func LocalQuery(idx *InvertedIndex, term string) []DocMeta {
	docIDs := idx.Query(term)
	var results []DocMeta
	for _, id := range docIDs {
		if doc, ok := idx.GetDoc(id); ok {
			results = append(results, doc)
		}
	}
	return results
}

// EncryptedQuery encrypts a search term using the index owner's public key,
// producing a ciphertext suitable for homomorphic comparison.
func EncryptedQuery(term string, encIdx *EncryptedIndex) (*rlwe.Ciphertext, error) {
	h := fhe.HashTerm(term)
	return fhe.EncryptSingleTerm(h, encIdx.Params, encIdx.PublicKey)
}

// DelegatedEvaluate performs homomorphic evaluation of an encrypted query
// against an encrypted index. This can be done by a peer who does NOT
// hold the secret key. The result is an encrypted match vector that only
// the index owner can decrypt.
//
// Returns one result ciphertext per term page.
func DelegatedEvaluate(encQuery *rlwe.Ciphertext, encIdx *EncryptedIndex) ([]*rlwe.Ciphertext, error) {
	var results []*rlwe.Ciphertext
	for _, page := range encIdx.TermPages {
		result, err := fhe.EvaluateMatch(encQuery, page.Ciphertext, encIdx.Params)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// DecryptResults decrypts the match vectors from DelegatedEvaluate and
// returns the slot indices where matches were found (zero-valued slots).
func DecryptResults(results []*rlwe.Ciphertext, encIdx *EncryptedIndex, keys *fhe.KeySet) ([]int, error) {
	var matchedSlots []int
	offset := 0

	for i, result := range results {
		activeSlots := encIdx.TermPages[i].SlotCount
		matches, err := fhe.DecryptMatchVector(result, keys.Params, keys.SecretKey, activeSlots)
		if err != nil {
			return nil, err
		}
		for j, m := range matches {
			if m {
				matchedSlots = append(matchedSlots, offset+j)
			}
		}
		offset += activeSlots
	}

	return matchedSlots, nil
}
