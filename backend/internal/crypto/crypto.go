// Package crypto provides authenticated symmetric encryption for secrets stored
// at rest (e.g. admin-supplied LLM API keys). It uses AES-256-GCM with a key
// derived from a deployment secret via SHA-256.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

// errMalformed is returned when ciphertext cannot be decrypted/authenticated.
var errMalformed = errors.New("crypto: malformed or tampered ciphertext")

// deriveKey turns an arbitrary secret string into a 32-byte AES key.
func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Encrypt seals plaintext with AES-256-GCM and returns base64(nonce||ciphertext).
func Encrypt(secret, plaintext string) (string, error) {
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. It fails if the secret is wrong or data was tampered.
func Decrypt(secret, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errMalformed
	}
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errMalformed
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", errMalformed
	}
	return string(plain), nil
}

// Last4 returns the last four characters of a secret for display (or the whole
// string if shorter), never revealing the full value.
func Last4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

// SHA256Hex returns the hex-encoded SHA-256 of s. Used to store API keys as a
// one-way hash (the raw key is high-entropy, so a fast hash is appropriate — no
// salt/bcrypt needed) and to look them up in constant DB time by hash.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
