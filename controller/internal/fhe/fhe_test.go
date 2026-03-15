package fhe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewParams(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	if params.N() == 0 {
		t.Fatal("expected non-zero ring degree")
	}
}

func TestNewParamsInvalid(t *testing.T) {
	_, err := NewParams(5)
	if err == nil {
		t.Error("expected error for logN=5")
	}
	_, err = NewParams(20)
	if err == nil {
		t.Error("expected error for logN=20")
	}
}

func TestGenerateKeys(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	ks, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}
	if ks.SecretKey == nil || ks.PublicKey == nil {
		t.Fatal("expected non-nil keys")
	}
}

func TestHashTerm(t *testing.T) {
	h1 := HashTerm("hello")
	h2 := HashTerm("Hello")
	h3 := HashTerm("world")

	if h1 != h2 {
		t.Error("expected case-insensitive hashing")
	}
	if h1 == h3 {
		t.Error("expected different hashes for different terms")
	}
	if h1 >= DefaultPlaintextModulus {
		t.Errorf("hash %d exceeds plaintext modulus %d", h1, DefaultPlaintextModulus)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	ks, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	hashes := []uint64{HashTerm("hello"), HashTerm("world"), HashTerm("test")}

	ct, err := EncryptTermPage(hashes, params, ks.PublicKey)
	if err != nil {
		t.Fatalf("EncryptTermPage: %v", err)
	}

	slots, err := DecryptSlots(ct, params, ks.SecretKey)
	if err != nil {
		t.Fatalf("DecryptSlots: %v", err)
	}

	for i, expected := range hashes {
		if slots[i] != expected {
			t.Errorf("slot %d: got %d, want %d", i, slots[i], expected)
		}
	}
}

func TestEvaluateMatchSameTerm(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	ks, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	termHash := HashTerm("searchterm")

	// Index page with the search term in slot 1.
	pageHashes := []uint64{HashTerm("other"), termHash, HashTerm("another")}
	encPage, err := EncryptTermPage(pageHashes, params, ks.PublicKey)
	if err != nil {
		t.Fatalf("EncryptTermPage: %v", err)
	}

	// Query: encrypt the search term replicated across all slots.
	encQuery, err := EncryptSingleTerm(termHash, params, ks.PublicKey)
	if err != nil {
		t.Fatalf("EncryptSingleTerm: %v", err)
	}

	// Evaluate: subtract query from page.
	result, err := EvaluateMatch(encQuery, encPage, params)
	if err != nil {
		t.Fatalf("EvaluateMatch: %v", err)
	}

	// Decrypt and check matches.
	matches, err := DecryptMatchVector(result, params, ks.SecretKey, len(pageHashes))
	if err != nil {
		t.Fatalf("DecryptMatchVector: %v", err)
	}

	if matches[0] {
		t.Error("slot 0 should NOT match")
	}
	if !matches[1] {
		t.Error("slot 1 SHOULD match (same term)")
	}
	if matches[2] {
		t.Error("slot 2 should NOT match")
	}
}

func TestEvaluateMatchNoMatch(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	ks, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	pageHashes := []uint64{HashTerm("alpha"), HashTerm("beta"), HashTerm("gamma")}
	encPage, err := EncryptTermPage(pageHashes, params, ks.PublicKey)
	if err != nil {
		t.Fatalf("EncryptTermPage: %v", err)
	}

	encQuery, err := EncryptSingleTerm(HashTerm("notfound"), params, ks.PublicKey)
	if err != nil {
		t.Fatalf("EncryptSingleTerm: %v", err)
	}

	result, err := EvaluateMatch(encQuery, encPage, params)
	if err != nil {
		t.Fatalf("EvaluateMatch: %v", err)
	}

	matches, err := DecryptMatchVector(result, params, ks.SecretKey, len(pageHashes))
	if err != nil {
		t.Fatalf("DecryptMatchVector: %v", err)
	}

	for i, m := range matches {
		if m {
			t.Errorf("slot %d should NOT match", i)
		}
	}
}

func TestKeySaveLoad(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	ks, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "keys")
	passphrase := "testpassword123"

	if err := ks.SaveKeys(dir, passphrase); err != nil {
		t.Fatalf("SaveKeys: %v", err)
	}

	// Verify files exist.
	for _, name := range []string{"pubkey.bin", "seckey.enc", "params.bin"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing key file %s: %v", name, err)
		}
	}

	// Load and verify round-trip.
	loaded, err := LoadKeys(dir, passphrase)
	if err != nil {
		t.Fatalf("LoadKeys: %v", err)
	}

	// Encrypt with original, decrypt with loaded.
	hashes := []uint64{HashTerm("roundtrip")}
	ct, err := EncryptTermPage(hashes, params, ks.PublicKey)
	if err != nil {
		t.Fatalf("EncryptTermPage: %v", err)
	}
	slots, err := DecryptSlots(ct, loaded.Params, loaded.SecretKey)
	if err != nil {
		t.Fatalf("DecryptSlots with loaded key: %v", err)
	}
	if slots[0] != hashes[0] {
		t.Errorf("round-trip mismatch: got %d, want %d", slots[0], hashes[0])
	}
}

func TestKeySaveLoadWrongPassphrase(t *testing.T) {
	params, err := NewParams(DefaultLogN)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	ks, err := GenerateKeys(params)
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "keys")
	if err := ks.SaveKeys(dir, "correct"); err != nil {
		t.Fatalf("SaveKeys: %v", err)
	}

	_, err = LoadKeys(dir, "wrong")
	if err == nil {
		t.Error("expected error with wrong passphrase")
	}
}

func TestHashPassphrase(t *testing.T) {
	h1 := HashPassphrase("test")
	h2 := HashPassphrase("test")
	h3 := HashPassphrase("different")

	if h1 != h2 {
		t.Error("same passphrase should produce same hash")
	}
	if h1 == h3 {
		t.Error("different passphrases should produce different hashes")
	}
}
