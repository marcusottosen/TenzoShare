package crypto_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tenzoshare/tenzoshare/shared/pkg/crypto"
)

// ── Encrypt / Decrypt ─────────────────────────────────────────────────────────

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	plaintext := []byte("hello tenzoshare")

	ct, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ct == "" {
		t.Fatal("expected non-empty ciphertext")
	}

	got, err := crypto.Decrypt(ct, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestEncryptDifferentKeyFails(t *testing.T) {
	key := bytes.Repeat([]byte("a"), 32)
	other := bytes.Repeat([]byte("b"), 32)

	ct, err := crypto.Encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	_, err = crypto.Decrypt(ct, other)
	if err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}

func TestEncryptWrongKeyLengthErrors(t *testing.T) {
	_, err := crypto.Encrypt([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecryptWrongKeyLengthErrors(t *testing.T) {
	_, err := crypto.Decrypt("dummyciphertext", []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestEncryptOutputIsDifferentEachCall(t *testing.T) {
	key := bytes.Repeat([]byte("x"), 32)
	msg := []byte("same message")

	ct1, _ := crypto.Encrypt(msg, key)
	ct2, _ := crypto.Encrypt(msg, key)
	if ct1 == ct2 {
		t.Fatal("encrypt should produce different ciphertexts (random nonce)")
	}
}

// ── HashPassword / VerifyPassword ─────────────────────────────────────────────

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := crypto.HashPassword("hunter2", "pepper")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "argon2id$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := crypto.VerifyPassword("hunter2", hash, "pepper")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("expected correct password to verify")
	}
}

func TestVerifyWrongPasswordFails(t *testing.T) {
	hash, _ := crypto.HashPassword("correct", "pepper")
	ok, err := crypto.VerifyPassword("wrong", hash, "pepper")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail verification")
	}
}

func TestVerifyWrongPepperFails(t *testing.T) {
	hash, _ := crypto.HashPassword("password", "correct-pepper")
	ok, err := crypto.VerifyPassword("password", hash, "wrong-pepper")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok {
		t.Fatal("expected wrong pepper to fail verification")
	}
}

func TestHashDifferentEachCall(t *testing.T) {
	h1, _ := crypto.HashPassword("pw", "p")
	h2, _ := crypto.HashPassword("pw", "p")
	if h1 == h2 {
		t.Fatal("hashes should differ due to random salt")
	}
}

// ── RandomBytes / RandomToken ─────────────────────────────────────────────────

func TestRandomBytesLength(t *testing.T) {
	for _, n := range []int{1, 16, 32, 64} {
		b, err := crypto.RandomBytes(n)
		if err != nil {
			t.Fatalf("RandomBytes(%d): %v", n, err)
		}
		if len(b) != n {
			t.Fatalf("expected %d bytes, got %d", n, len(b))
		}
	}
}

func TestRandomBytesAreRandom(t *testing.T) {
	b1, _ := crypto.RandomBytes(32)
	b2, _ := crypto.RandomBytes(32)
	if bytes.Equal(b1, b2) {
		t.Fatal("two calls should not produce identical bytes")
	}
}

func TestRandomTokenIsURLSafe(t *testing.T) {
	tok, err := crypto.RandomToken(32)
	if err != nil {
		t.Fatalf("RandomToken: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	// URL-safe base64 should not contain + or /
	if strings.ContainsAny(tok, "+/") {
		t.Fatalf("token contains URL-unsafe chars: %q", tok)
	}
}
