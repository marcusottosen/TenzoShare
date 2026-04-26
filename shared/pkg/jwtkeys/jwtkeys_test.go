package jwtkeys_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/tenzoshare/tenzoshare/shared/pkg/jwtkeys"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func generateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func encodePKCS8Private(t *testing.T, k *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func encodePKCS1Private(k *rsa.PrivateKey) string {
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k),
	}))
}

func encodePublic(t *testing.T, k *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// ── ParsePrivateKey ───────────────────────────────────────────────────────────

func TestParsePrivateKey_PKCS8_Roundtrip(t *testing.T) {
	k := generateKey(t)
	pem := encodePKCS8Private(t, k)

	parsed, err := jwtkeys.ParsePrivateKey(pem)
	if err != nil {
		t.Fatalf("ParsePrivateKey PKCS8: %v", err)
	}
	if parsed.N.Cmp(k.N) != 0 {
		t.Fatal("parsed key modulus mismatch")
	}
}

func TestParsePrivateKey_PKCS1_Roundtrip(t *testing.T) {
	k := generateKey(t)
	pem := encodePKCS1Private(k)

	parsed, err := jwtkeys.ParsePrivateKey(pem)
	if err != nil {
		t.Fatalf("ParsePrivateKey PKCS1: %v", err)
	}
	if parsed.N.Cmp(k.N) != 0 {
		t.Fatal("parsed key modulus mismatch")
	}
}

func TestParsePrivateKey_Empty_ReturnsError(t *testing.T) {
	_, err := jwtkeys.ParsePrivateKey("")
	if err == nil {
		t.Fatal("expected error for empty PEM")
	}
}

func TestParsePrivateKey_InvalidPEM_ReturnsError(t *testing.T) {
	_, err := jwtkeys.ParsePrivateKey("not a pem block")
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestParsePrivateKey_PublicKeyAsPEM_ReturnsError(t *testing.T) {
	k := generateKey(t)
	pubPEM := encodePublic(t, k)
	// Passing a public key PEM where a private key is expected should fail
	_, err := jwtkeys.ParsePrivateKey(pubPEM)
	if err == nil {
		t.Fatal("expected error when passing public key as private key")
	}
}

// ── ParsePublicKey ────────────────────────────────────────────────────────────

func TestParsePublicKey_Roundtrip(t *testing.T) {
	k := generateKey(t)
	pubPEM := encodePublic(t, k)

	parsed, err := jwtkeys.ParsePublicKey(pubPEM)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}
	if parsed.N.Cmp(k.N) != 0 {
		t.Fatal("parsed public key modulus mismatch")
	}
}

func TestParsePublicKey_Empty_ReturnsError(t *testing.T) {
	_, err := jwtkeys.ParsePublicKey("")
	if err == nil {
		t.Fatal("expected error for empty PEM")
	}
}

func TestParsePublicKey_InvalidPEM_ReturnsError(t *testing.T) {
	_, err := jwtkeys.ParsePublicKey("garbage")
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestParsePublicKey_PrivateKeyAsPEM_ReturnsError(t *testing.T) {
	k := generateKey(t)
	privPEM := encodePKCS8Private(t, k)
	// Passing a private key PEM where a public key is expected should fail
	_, err := jwtkeys.ParsePublicKey(privPEM)
	if err == nil {
		t.Fatal("expected error when passing private key as public key")
	}
}

// ── Key pair compatibility ────────────────────────────────────────────────────

func TestKeyPair_SignVerify(t *testing.T) {
	// Verify that keys parsed by jwtkeys can actually be used for JWT signing/verification
	k := generateKey(t)
	privPEM := encodePKCS8Private(t, k)
	pubPEM := encodePublic(t, k)

	priv, err := jwtkeys.ParsePrivateKey(privPEM)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := jwtkeys.ParsePublicKey(pubPEM)
	if err != nil {
		t.Fatal(err)
	}

	// Confirm the public keys match
	if priv.PublicKey.N.Cmp(pub.N) != 0 {
		t.Fatal("public key from private does not match parsed public key")
	}
}
