package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// secretBox encrypts stored Meta secrets (business tokens, registration PINs)
// with AES-256-GCM. The stored value is nonce || ciphertext.
type secretBox struct {
	aead cipher.AEAD
}

func newSecretBox(key []byte) (*secretBox, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("token encryption key: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("token encryption: %w", err)
	}
	return &secretBox{aead: aead}, nil
}

func (b *secretBox) seal(plain string) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return b.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (b *secretBox) open(sealed []byte) (string, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("sealed secret too short")
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], nil)
	if err != nil {
		return "", fmt.Errorf("open secret: %w", err)
	}
	return string(plain), nil
}
