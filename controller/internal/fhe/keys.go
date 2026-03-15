package fhe

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
	"golang.org/x/crypto/argon2"
)

// KeySet holds the FHE key material for a single identity.
type KeySet struct {
	Params        bgv.Parameters
	SecretKey     *rlwe.SecretKey
	PublicKey     *rlwe.PublicKey
	EvaluationKey rlwe.EvaluationKeySet // for multiplication/relinearization (may be nil)
}

// GenerateKeys creates a new FHE key pair with optional evaluation keys
// for homomorphic multiplication (required for vector dot product).
func GenerateKeys(params bgv.Parameters) (*KeySet, error) {
	kgen := rlwe.NewKeyGenerator(params)
	sk, pk := kgen.GenKeyPairNew()

	// Generate relinearization key for ciphertext × ciphertext multiplication.
	rlk := kgen.GenRelinearizationKeyNew(sk)
	evk := rlwe.NewMemEvaluationKeySet(rlk)

	return &KeySet{
		Params:        params,
		SecretKey:     sk,
		PublicKey:     pk,
		EvaluationKey: evk,
	}, nil
}

// MarshalPublicKey serializes the public key to bytes.
func (ks *KeySet) MarshalPublicKey() ([]byte, error) {
	return ks.PublicKey.MarshalBinary()
}

// MarshalSecretKey serializes the secret key to bytes.
func (ks *KeySet) MarshalSecretKey() ([]byte, error) {
	return ks.SecretKey.MarshalBinary()
}

// SaveKeys writes the key set to disk. The secret key is encrypted
// with AES-256-GCM using a key derived from the passphrase via Argon2id.
func (ks *KeySet) SaveKeys(dir, passphrase string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create key dir: %w", err)
	}

	// Save public key (unencrypted — safe to share).
	pkBytes, err := ks.MarshalPublicKey()
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}
	if err := os.WriteFile(dir+"/pubkey.bin", pkBytes, 0644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	// Save secret key (encrypted at rest).
	skBytes, err := ks.MarshalSecretKey()
	if err != nil {
		return fmt.Errorf("marshal secret key: %w", err)
	}

	encrypted, err := encryptWithPassphrase(skBytes, passphrase)
	if err != nil {
		return fmt.Errorf("encrypt secret key: %w", err)
	}
	if err := os.WriteFile(dir+"/seckey.enc", encrypted, 0600); err != nil {
		return fmt.Errorf("write secret key: %w", err)
	}

	// Save params for reconstruction.
	paramsBytes, err := ks.Params.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	if err := os.WriteFile(dir+"/params.bin", paramsBytes, 0644); err != nil {
		return fmt.Errorf("write params: %w", err)
	}

	return nil
}

// LoadKeys reads a key set from disk, decrypting the secret key with the passphrase.
func LoadKeys(dir, passphrase string) (*KeySet, error) {
	// Load params.
	paramsBytes, err := os.ReadFile(dir + "/params.bin")
	if err != nil {
		return nil, fmt.Errorf("read params: %w", err)
	}
	var params bgv.Parameters
	if err := params.UnmarshalBinary(paramsBytes); err != nil {
		return nil, fmt.Errorf("unmarshal params: %w", err)
	}

	// Load public key.
	pkBytes, err := os.ReadFile(dir + "/pubkey.bin")
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	pk := rlwe.NewPublicKey(params)
	if err := pk.UnmarshalBinary(pkBytes); err != nil {
		return nil, fmt.Errorf("unmarshal public key: %w", err)
	}

	// Load and decrypt secret key.
	encBytes, err := os.ReadFile(dir + "/seckey.enc")
	if err != nil {
		return nil, fmt.Errorf("read secret key: %w", err)
	}
	skBytes, err := decryptWithPassphrase(encBytes, passphrase)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret key: %w", err)
	}
	sk := rlwe.NewSecretKey(params)
	if err := sk.UnmarshalBinary(skBytes); err != nil {
		return nil, fmt.Errorf("unmarshal secret key: %w", err)
	}

	return &KeySet{
		Params:    params,
		SecretKey: sk,
		PublicKey: pk,
	}, nil
}

// encryptWithPassphrase encrypts data using AES-256-GCM with an Argon2id-derived key.
// Output format: salt (16 bytes) || nonce (12 bytes) || ciphertext.
func encryptWithPassphrase(data []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key := argon2.IDKey([]byte(passphrase), salt, 3, 64*1024, 4, 32)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)
	result := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// decryptWithPassphrase decrypts data encrypted by encryptWithPassphrase.
func decryptWithPassphrase(data []byte, passphrase string) ([]byte, error) {
	if len(data) < 28 { // 16 salt + 12 nonce minimum
		return nil, fmt.Errorf("encrypted data too short")
	}

	salt := data[:16]
	key := argon2.IDKey([]byte(passphrase), salt, 3, 64*1024, 4, 32)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < 16+nonceSize {
		return nil, fmt.Errorf("encrypted data too short for nonce")
	}
	nonce := data[16 : 16+nonceSize]
	ciphertext := data[16+nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// HashPassphrase returns a SHA-256 hash of the passphrase for config storage
// (used to verify the passphrase without storing it).
func HashPassphrase(passphrase string) string {
	h := sha256.Sum256([]byte(passphrase))
	return fmt.Sprintf("%x", h)
}
