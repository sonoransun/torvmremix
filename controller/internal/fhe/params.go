// Package fhe provides Fully Homomorphic Encryption primitives for
// encrypted document search indexing using the BFV scheme via Lattigo.
package fhe

import (
	"fmt"

	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

// DefaultPlaintextModulus is a prime that fits search term hashes.
// 65537 is a Fermat prime, coprime with all standard ciphertext moduli.
const DefaultPlaintextModulus = 0x10001 // 65537

// SlotsPerCiphertext returns the number of SIMD plaintext slots for the
// given ring degree. Each ciphertext can hold N plaintext values.
func SlotsPerCiphertext(logN int) int {
	return 1 << logN
}

// NewParams creates BFV parameters with the given ring degree.
// logN=12 (N=4096) provides 128-bit security with ~32KB ciphertexts.
// logN=11 (N=2048) is faster but lower security (~100 bits).
func NewParams(logN int) (bgv.Parameters, error) {
	if logN < 10 || logN > 15 {
		return bgv.Parameters{}, fmt.Errorf("logN must be 10-15, got %d", logN)
	}

	var logQ []int
	var logP []int

	switch {
	case logN >= 14:
		logQ = []int{56, 55, 55, 54, 54}
		logP = []int{55, 55}
	case logN >= 12:
		logQ = []int{40, 40, 40}
		logP = []int{40}
	default:
		logQ = []int{30, 30}
		logP = []int{30}
	}

	params, err := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             logQ,
		LogP:             logP,
		PlaintextModulus: DefaultPlaintextModulus,
	})
	if err != nil {
		return bgv.Parameters{}, fmt.Errorf("create BFV params: %w", err)
	}

	return params, nil
}

// DefaultLogN is the default ring degree exponent (N=4096, 128-bit security).
const DefaultLogN = 12
