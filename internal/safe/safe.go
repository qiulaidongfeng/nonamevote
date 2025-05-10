// Package safe 提供安全相关功能
package safe

import (
	"crypto/aes"
	"crypto/cipher"
	"os"
	"unsafe"

	"golang.org/x/crypto/argon2"
)

var gcm cipher.AEAD
var Aeskey [32]byte

func init() {
	main_key := os.Getenv("main_key")
	if main_key == "" {
		panic("环境变量main_key应该提供主密钥")
	}
	s := salt()
	aes_key := argon2.IDKey([]byte(main_key), s, 2, 64*1024, 4, 32)
	block, err := aes.NewCipher(aes_key)
	if err != nil {
		panic(err)
	}
	Aeskey = [32]byte(aes_key)
	gcm, err = cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		panic(err)
	}
}

func Encrypt(v string) string {
	ev := gcm.Seal(nil, nil, unsafe.Slice(unsafe.StringData(v), len(v)), nil)
	return unsafe.String(unsafe.SliceData(ev), len(ev))
}

func Decrypt(v string) string {
	ev, _ := gcm.Open(nil, nil, unsafe.Slice(unsafe.StringData(v), len(v)), nil)
	return unsafe.String(unsafe.SliceData(ev), len(ev))
}
