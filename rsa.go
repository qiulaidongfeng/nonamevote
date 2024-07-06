package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
)

var (
	pubkey  *rsa.PublicKey
	privkey *rsa.PrivateKey
)

func initRSA() {
	d, err := os.ReadFile("rsa.pem")
	if err != nil {
		if os.IsNotExist(err) {
			genRSA()
			return
		}
		panic(err)
	}
	block, _ := pem.Decode(d)
	privkey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	pubkey = &privkey.PublicKey
}

func genRSA() {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		panic(err)
	}
	pubkey = &key.PublicKey
	privkey = key

	rsaOut, err := os.Create("rsa.pem")
	if err != nil {
		panic(err)
	}
	priv := x509.MarshalPKCS1PrivateKey(privkey)
	pem.Encode(rsaOut, &pem.Block{Type: "Private key", Bytes: priv})
	rsaOut.Close()
}
