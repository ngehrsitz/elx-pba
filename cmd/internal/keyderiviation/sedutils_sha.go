package keyderiviation

import (
	"crypto/sha1"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

type SedutilSha struct{}

func (s SedutilSha) DeriveKey(password string, driveSerial []byte) []byte {
	salt := fmt.Sprintf("%-20s", string(driveSerial))
	return pbkdf2.Key([]byte(password), []byte(salt[:20]), 75000, 32, sha1.New)
}
