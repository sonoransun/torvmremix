package fhe

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

// HashTerm normalizes and hashes a search term to a plaintext slot value.
// The result fits within the BFV plaintext modulus.
func HashTerm(term string) uint64 {
	normalized := strings.ToLower(strings.TrimSpace(term))
	h := sha256.Sum256([]byte(normalized))
	v := binary.LittleEndian.Uint64(h[:8])
	return v % DefaultPlaintextModulus
}

// EncryptTermPage encrypts a batch of term hashes into a single ciphertext
// using SIMD slot packing. Up to N slots can be packed per ciphertext.
func EncryptTermPage(hashes []uint64, params bgv.Parameters, pk *rlwe.PublicKey) (*rlwe.Ciphertext, error) {
	encoder := bgv.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, pk)

	n := params.N()
	slots := make([]uint64, n)
	for i := 0; i < len(hashes) && i < n; i++ {
		slots[i] = hashes[i]
	}

	pt := bgv.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(slots, pt); err != nil {
		return nil, err
	}

	ct, err := encryptor.EncryptNew(pt)
	if err != nil {
		return nil, err
	}
	return ct, nil
}

// EncryptSingleTerm encrypts a single term hash, replicated across all
// SIMD slots for parallel comparison against a term page.
func EncryptSingleTerm(hash uint64, params bgv.Parameters, pk *rlwe.PublicKey) (*rlwe.Ciphertext, error) {
	encoder := bgv.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, pk)

	n := params.N()
	slots := make([]uint64, n)
	for i := range slots {
		slots[i] = hash
	}

	pt := bgv.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(slots, pt); err != nil {
		return nil, err
	}

	ct, err := encryptor.EncryptNew(pt)
	if err != nil {
		return nil, err
	}
	return ct, nil
}

// DecryptSlots decrypts a ciphertext and returns the plaintext slot values.
func DecryptSlots(ct *rlwe.Ciphertext, params bgv.Parameters, sk *rlwe.SecretKey) ([]uint64, error) {
	decryptor := rlwe.NewDecryptor(params, sk)
	encoder := bgv.NewEncoder(params)

	pt := decryptor.DecryptNew(ct)

	n := params.N()
	slots := make([]uint64, n)
	if err := encoder.Decode(pt, slots); err != nil {
		return nil, err
	}
	return slots, nil
}
