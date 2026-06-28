package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "deployment-secret"
	plain := "sk-proj-abcdef1234567890"
	ct, err := Encrypt(secret, plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ct, plain) {
		t.Fatal("ciphertext leaked the plaintext")
	}
	got, err := Decrypt(secret, ct)
	if err != nil || got != plain {
		t.Fatalf("round trip failed: got=%q err=%v", got, err)
	}
	// Two encryptions of the same value differ (random nonce).
	ct2, _ := Encrypt(secret, plain)
	if ct == ct2 {
		t.Fatal("expected distinct ciphertexts (nonce reuse?)")
	}
}

func TestDecryptFailures(t *testing.T) {
	ct, _ := Encrypt("right", "value")
	if _, err := Decrypt("wrong", ct); err == nil {
		t.Fatal("wrong secret should fail")
	}
	if _, err := Decrypt("right", "!!!not-base64!!!"); err == nil {
		t.Fatal("malformed base64 should fail")
	}
	if _, err := Decrypt("right", "QUJD"); err == nil { // valid base64, too short for nonce
		t.Fatal("short ciphertext should fail")
	}
	// Tampered ciphertext fails GCM auth.
	tampered := ct[:len(ct)-2] + "AA"
	if _, err := Decrypt("right", tampered); err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
}

func TestLast4(t *testing.T) {
	if Last4("abcdefgh") != "efgh" {
		t.Fatalf("last4: %s", Last4("abcdefgh"))
	}
	if Last4("ab") != "ab" {
		t.Fatal("short returns whole")
	}
}
