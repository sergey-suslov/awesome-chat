package application

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/sergey-suslov/awesome-chat/common/types"
)

type UserInfoLocal struct {
	ui  types.UserInfo
	pub *rsa.PublicKey
}

func NewUserInfoLocal(ui types.UserInfo) *UserInfoLocal {
	block, _ := pem.Decode(ui.Pub)
	if block == nil {
		panic("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("failed to parse DER encoded public key: " + err.Error())
	}

	pubKey := pub.(*rsa.PublicKey)
	return &UserInfoLocal{ui: ui, pub: pubKey}
}
