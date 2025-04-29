package security

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func TestAESEncryptDecrypt_Success(t *testing.T) {
	ts := time.Now().UnixNano()

	b := []byte(strconv.FormatInt(ts, 10))

	fmt.Println(len(b), "??????????")

	password := []byte("SuperSecretPassword123!")
	in := []byte("Hello, this is a secret message.")

	fmt.Println("plaintext:", string(in))

	cipherData, err := EncryptAES(in, password)
	assert.NoError(t, err)

	out, err := decryptAES(cipherData, password)
	assert.NoError(t, err)

	fmt.Println("decrypted:", string(out))

	assert.Equal(t, in, out)
}
