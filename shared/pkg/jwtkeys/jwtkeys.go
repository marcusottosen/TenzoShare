// Package jwtkeys provides helpers to parse RSA keys from PEM strings
// stored in environment variables (newlines escaped as \n).
package jwtkeys

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// ParsePublicKey parses a PEM-encoded RSA public key.
// The PEM string may have literal \n escape sequences (as stored in env vars).
func ParsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	if pemStr == "" {
		return nil, fmt.Errorf("JWT_PUBLIC_KEY is not set")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("jwtkeys: failed to decode public key PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwtkeys: parse public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("jwtkeys: public key is not RSA")
	}
	return rsaKey, nil
}

// ParsePrivateKey parses a PKCS#8 or PKCS#1 PEM-encoded RSA private key.
func ParsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	if pemStr == "" {
		return nil, fmt.Errorf("JWT_PRIVATE_KEY is not set")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("jwtkeys: failed to decode private key PEM block")
	}
	// Try PKCS#8 first (openssl genpkey output)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("jwtkeys: PKCS#8 key is not RSA")
		}
		return rsaKey, nil
	}
	// Fall back to PKCS#1
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
