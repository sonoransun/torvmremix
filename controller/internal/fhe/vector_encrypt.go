package fhe

import (
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

// DefaultScale is the fixed-point quantization scale for converting
// float32 vector components to integer plaintext values.
// Scale of 100 is chosen so that per-slot products (scale^2 * 1.0 = 10000)
// stay well below the plaintext modulus T=65537, avoiding modular overflow
// during homomorphic multiplication. Provides 2 decimal places of precision,
// sufficient for cosine similarity ranking.
const DefaultScale = 100.0

// QuantizeVector converts a float32 vector to uint64 values suitable
// for BFV/BGV plaintext. Each component v becomes round(v * scale) mod T.
func QuantizeVector(vec []float32, scale float64) []uint64 {
	result := make([]uint64, len(vec))
	for i, v := range vec {
		// Scale and round to nearest integer.
		scaled := math.Round(float64(v) * scale)
		// Map to positive range [0, T) for plaintext modulus.
		if scaled < 0 {
			scaled += float64(DefaultPlaintextModulus)
		}
		result[i] = uint64(scaled) % DefaultPlaintextModulus
	}
	return result
}

// DequantizeVector reverses the quantization, converting uint64 plaintext
// values back to float32 vector components.
func DequantizeVector(vals []uint64, scale float64, dim int) []float32 {
	result := make([]float32, dim)
	halfT := uint64(DefaultPlaintextModulus / 2)
	for i := 0; i < dim && i < len(vals); i++ {
		v := vals[i]
		// Values > T/2 are negative in signed representation.
		if v > halfT {
			result[i] = -float32(float64(DefaultPlaintextModulus-v) / scale)
		} else {
			result[i] = float32(float64(v) / scale)
		}
	}
	return result
}

// EncryptVector encrypts a quantized vector by packing components into
// SIMD slots of a single ciphertext.
func EncryptVector(vec []uint64, params bgv.Parameters, pk *rlwe.PublicKey) (*rlwe.Ciphertext, error) {
	return EncryptTermPage(vec, params, pk)
}

// EncryptVectorBatch encrypts multiple quantized vectors, each as a
// separate ciphertext.
func EncryptVectorBatch(vecs [][]uint64, params bgv.Parameters, pk *rlwe.PublicKey) ([]*rlwe.Ciphertext, error) {
	results := make([]*rlwe.Ciphertext, len(vecs))
	for i, vec := range vecs {
		ct, err := EncryptVector(vec, params, pk)
		if err != nil {
			return nil, err
		}
		results[i] = ct
	}
	return results, nil
}

// DecryptVector decrypts a ciphertext and dequantizes back to float32.
func DecryptVector(ct *rlwe.Ciphertext, params bgv.Parameters, sk *rlwe.SecretKey, scale float64, dim int) ([]float32, error) {
	vals, err := DecryptSlots(ct, params, sk)
	if err != nil {
		return nil, err
	}
	return DequantizeVector(vals, scale, dim), nil
}
