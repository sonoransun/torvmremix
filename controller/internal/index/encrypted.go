package index

import (
	"crypto/sha256"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"

	"github.com/user/extorvm/controller/internal/fhe"
)

// EncryptedPage holds a single ciphertext containing up to N SIMD-packed
// term hashes or document ID encodings.
type EncryptedPage struct {
	Ciphertext *rlwe.Ciphertext
	SlotCount  int // number of active slots in this page
}

// EncryptedIndex is an FHE-encrypted inverted index that can be shared
// with peers without revealing the indexed terms or documents.
type EncryptedIndex struct {
	Params     bgv.Parameters
	PublicKey  *rlwe.PublicKey
	TermPages  []EncryptedPage
	PageCount  int
	TermCount  int
	DocCount   int
	Checksum   [32]byte // SHA-256 of serialized term page ciphertexts
}

// BuildEncryptedIndex encrypts a plaintext inverted index using the
// provided FHE key set. Terms are packed into SIMD pages with N
// hashes per ciphertext.
func BuildEncryptedIndex(idx *InvertedIndex, keys *fhe.KeySet) (*EncryptedIndex, error) {
	params := keys.Params
	slotsPerPage := params.N()

	allHashes := idx.TermHashes()

	var pages []EncryptedPage
	hasher := sha256.New()

	for i := 0; i < len(allHashes); i += slotsPerPage {
		end := i + slotsPerPage
		if end > len(allHashes) {
			end = len(allHashes)
		}
		pageHashes := allHashes[i:end]

		ct, err := fhe.EncryptTermPage(pageHashes, params, keys.PublicKey)
		if err != nil {
			return nil, err
		}

		// Contribute to checksum.
		ctBytes, err := ct.MarshalBinary()
		if err != nil {
			return nil, err
		}
		hasher.Write(ctBytes)

		pages = append(pages, EncryptedPage{
			Ciphertext: ct,
			SlotCount:  len(pageHashes),
		})
	}

	var checksum [32]byte
	copy(checksum[:], hasher.Sum(nil))

	return &EncryptedIndex{
		Params:    params,
		PublicKey: keys.PublicKey,
		TermPages: pages,
		PageCount: len(pages),
		TermCount: idx.TermCount(),
		DocCount:  idx.DocCount(),
		Checksum:  checksum,
	}, nil
}
