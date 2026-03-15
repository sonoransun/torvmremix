package fhe

import (
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

// EvaluateElementWiseProduct computes the homomorphic element-wise product
// of two encrypted vectors. The result ciphertext contains the product
// of corresponding SIMD slots. To get the dot product, the caller must
// sum the slots after decryption.
//
// This requires evaluation keys (relinearization) since it involves
// ciphertext × ciphertext multiplication.
func EvaluateElementWiseProduct(encA, encB *rlwe.Ciphertext,
	params bgv.Parameters, evk rlwe.EvaluationKeySet) (*rlwe.Ciphertext, error) {

	eval := bgv.NewEvaluator(params, evk)

	result, err := eval.MulRelinNew(encA, encB)
	if err != nil {
		return nil, fmt.Errorf("homomorphic multiplication: %w", err)
	}

	return result, nil
}

// DecryptDotProduct decrypts an element-wise product ciphertext, sums
// the slot values to produce the dot product, and scales back to float32.
func DecryptDotProduct(result *rlwe.Ciphertext, params bgv.Parameters,
	sk *rlwe.SecretKey, scale float64, dim int) (float32, error) {

	slots, err := DecryptSlots(result, params, sk)
	if err != nil {
		return 0, err
	}

	// Sum slot values (each is a_i * b_i in quantized form).
	var sum int64
	halfT := uint64(DefaultPlaintextModulus / 2)
	for i := 0; i < dim && i < len(slots); i++ {
		v := slots[i]
		if v > halfT {
			sum -= int64(DefaultPlaintextModulus - v)
		} else {
			sum += int64(v)
		}
	}

	// The dot product was computed on quantized values (each * scale),
	// so the result is scaled by scale^2.
	return float32(float64(sum) / (scale * scale)), nil
}
