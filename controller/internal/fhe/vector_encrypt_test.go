package fhe

import (
	"math"
	"testing"
)

func TestQuantizeDequantizeRoundTrip(t *testing.T) {
	vec := []float32{0.5, -0.3, 0.0, 0.9999, -0.9999}
	scale := DefaultScale

	quantized := QuantizeVector(vec, scale)
	recovered := DequantizeVector(quantized, scale, len(vec))

	for i, expected := range vec {
		diff := math.Abs(float64(recovered[i] - expected))
		if diff > 0.001 {
			t.Errorf("slot %d: expected %.4f, got %.4f (diff %.6f)", i, expected, recovered[i], diff)
		}
	}
}

func TestQuantizePositiveOnly(t *testing.T) {
	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	quantized := QuantizeVector(vec, DefaultScale)

	for i, q := range quantized {
		if q >= DefaultPlaintextModulus {
			t.Errorf("slot %d: quantized value %d exceeds plaintext modulus %d", i, q, DefaultPlaintextModulus)
		}
	}
}

func TestEncryptDecryptVectorRoundTrip(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	keys, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	original := []float32{0.5, -0.3, 0.1, 0.8}
	quantized := QuantizeVector(original, DefaultScale)

	ct, err := EncryptVector(quantized, params, keys.PublicKey)
	if err != nil {
		t.Fatalf("EncryptVector: %v", err)
	}

	recovered, err := DecryptVector(ct, params, keys.SecretKey, DefaultScale, len(original))
	if err != nil {
		t.Fatalf("DecryptVector: %v", err)
	}

	for i, expected := range original {
		diff := math.Abs(float64(recovered[i] - expected))
		if diff > 0.001 {
			t.Errorf("slot %d: expected %.4f, got %.4f", i, expected, recovered[i])
		}
	}
}

func TestHomomorphicDotProduct(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	keys, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	a := []float32{0.5, 0.3, 0.1}
	b := []float32{0.2, 0.4, 0.6}

	// Expected dot product: 0.5*0.2 + 0.3*0.4 + 0.1*0.6 = 0.1 + 0.12 + 0.06 = 0.28
	expectedDot := float32(0.28)

	// Use a small scale to avoid modular overflow: max product per slot
	// is scale^2 * 1.0 = 100^2 = 10000, well under T=65537.
	scale := 100.0

	qA := QuantizeVector(a, scale)
	qB := QuantizeVector(b, scale)

	ctA, err := EncryptVector(qA, params, keys.PublicKey)
	if err != nil {
		t.Fatalf("EncryptVector A: %v", err)
	}
	ctB, err := EncryptVector(qB, params, keys.PublicKey)
	if err != nil {
		t.Fatalf("EncryptVector B: %v", err)
	}

	product, err := EvaluateElementWiseProduct(ctA, ctB, params, keys.EvaluationKey)
	if err != nil {
		t.Fatalf("EvaluateElementWiseProduct: %v", err)
	}

	score, err := DecryptDotProduct(product, params, keys.SecretKey, scale, len(a))
	if err != nil {
		t.Fatalf("DecryptDotProduct: %v", err)
	}

	diff := math.Abs(float64(score - expectedDot))
	t.Logf("dot product: expected %.4f, got %.4f (diff %.6f)", expectedDot, score, diff)
	if diff > 0.01 {
		t.Errorf("dot product too far from expected: %.4f vs %.4f", score, expectedDot)
	}
}
