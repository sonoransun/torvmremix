package fhe

import (
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

// EvaluateMatch computes the homomorphic subtraction of an encrypted query
// term from an encrypted term page. In the resulting ciphertext, slots
// where the value is zero indicate a match (query term == index term).
//
// This requires no evaluation/relinearization keys — subtraction is a
// level-0 operation in BFV/BGV.
func EvaluateMatch(encQuery, encTermPage *rlwe.Ciphertext, params bgv.Parameters) (*rlwe.Ciphertext, error) {
	eval := bgv.NewEvaluator(params, nil)

	result, err := eval.SubNew(encQuery, encTermPage)
	if err != nil {
		return nil, fmt.Errorf("homomorphic subtraction: %w", err)
	}

	return result, nil
}

// DecryptMatchVector decrypts the result of EvaluateMatch and returns
// a boolean slice where true indicates a matching slot (value == 0).
func DecryptMatchVector(result *rlwe.Ciphertext, params bgv.Parameters, sk *rlwe.SecretKey, activeSlots int) ([]bool, error) {
	slots, err := DecryptSlots(result, params, sk)
	if err != nil {
		return nil, err
	}

	matches := make([]bool, activeSlots)
	for i := 0; i < activeSlots && i < len(slots); i++ {
		matches[i] = slots[i] == 0
	}
	return matches, nil
}
