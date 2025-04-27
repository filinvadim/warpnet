package security

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestArgon2idEncryptDecrypt_Success(t *testing.T) {
	password := []byte("SuperSecretPassword123!")
	in := []byte("Hello, this is a secret message.")

	c := NewArgon2Cipher(&Argon2Params{
		Time:    60,
		Memory:  32,
		KeyLen:  32,
		SaltLen: 32,
		Threads: 1,
	})

	fmt.Println("plaintext:", string(in))

	cipherData, err := c.EncryptA2id(in, password)
	assert.NoError(t, err)

	out, err := c.decryptA2id(cipherData, password)
	assert.NoError(t, err)

	fmt.Println("decrypted:", string(out))

	assert.Equal(t, in, out)
}
