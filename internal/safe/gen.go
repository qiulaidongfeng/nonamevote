package safe

import (
	"crypto/rand"
	"os"
)

func salt() []byte {
	salt, err := os.ReadFile("./salt")
	if err != nil {
		if os.IsNotExist(err) {
			var salt [32]byte
			rand.Read(salt[:])
			err := os.WriteFile("./salt", salt[:], 0600)
			if err != nil {
				panic(err)
			}
			return salt[:]
		}
		panic(err)
	}
	return salt
}
