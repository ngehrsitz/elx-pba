package keyderiviation

type KeyDerivationFunction interface {
	DeriveKey(password string, driveSerial []byte) []byte
}
