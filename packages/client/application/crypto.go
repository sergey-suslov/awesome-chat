package application

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
)

type KeyPair struct {
	priv *rsa.PrivateKey
}

func GenerateKeyPair() (*KeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}
	return &KeyPair{priv: privateKey}, nil
}

func (kp *KeyPair) DerivePublicKey() *rsa.PublicKey {
	return &kp.priv.PublicKey
}

func (kp *KeyPair) PublicKeyToX509() ([]byte, error) {
	pub, err := x509.MarshalPKIXPublicKey(kp.DerivePublicKey())
	if err != nil {
		return nil, err
	}
	encoded := pem.EncodeToMemory(&pem.Block{
		Type:    "PUBLIC KEY",
		Headers: map[string]string{},
		Bytes:   pub,
	})
	return encoded, nil
}

func (kp *KeyPair) DecryptWithPriv(ciphertext []byte, label []byte) ([]byte, error) {
	plaintext, err := rsa.DecryptOAEP(sha256.New(), nil, kp.priv, ciphertext, label)

	if err != nil {
		return nil, err
	}
	return plaintext, err
}

func (kp *KeyPair) EncryptWithPub(plaintext []byte, label []byte, pub *rsa.PublicKey) ([]byte, error) {
	plaintext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, plaintext, label)

	if err != nil {
		return nil, err
	}
	return plaintext, err
}
