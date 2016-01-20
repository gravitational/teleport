package lunk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"unsafe"
)

// An ID is a unique, uniformly distributed 64-bit ID.
type ID uint64

// String returns the ID as a hex string.
func (id ID) String() string {
	return fmt.Sprintf("%016x", uint64(id))
}

// MarshalJSON encodes the ID as a hex string.
func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

// UnmarshalJSON decodes the given data as either a hexadecimal string or JSON
// integer.
func (id *ID) UnmarshalJSON(data []byte) error {
	i, err := parseJSONString(data)
	if err == nil {
		*id = i
		return nil
	}

	i, err = parseJSONInt(data)
	if err == nil {
		*id = i
		return nil
	}

	return fmt.Errorf("%s is not a valid ID", data)
}

// ParseID parses the given string as a hexadecimal string.
func ParseID(s string) (ID, error) {
	i, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0, err
	}
	return ID(i), nil
}

// generateID returns a randomly-generated 64-bit ID. This function is
// thread-safe.  IDs are produced by consuming an AES-CTR-128 keystream in
// 64-bit chunks. The AES key is randomly generated on initialization, as is the
// counter's initial state. On machines with AES-NI support, ID generation takes
// ~30ns and generates no garbage.
func generateID() ID {
	m.Lock()
	if n == aes.BlockSize {
		c.Encrypt(b, ctr)
		for i := aes.BlockSize - 1; i >= 0; i-- { // increment ctr
			ctr[i]++
			if ctr[i] != 0 {
				break
			}
		}
		n = 0
	}
	id := *(*ID)(unsafe.Pointer(&b[n])) // zero-copy b/c we're arch-neutral
	n += idSize
	m.Unlock()

	return id
}

const (
	idSize  = aes.BlockSize / 2 // 64 bits
	keySize = aes.BlockSize     // 128 bits
)

var (
	ctr []byte
	n   int
	b   []byte
	c   cipher.Block
	m   sync.Mutex
)

func init() {
	buf := make([]byte, keySize+aes.BlockSize)
	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		panic(err) // /dev/urandom had better work
	}

	c, err = aes.NewCipher(buf[:keySize])
	if err != nil {
		panic(err) // AES had better work
	}

	n = aes.BlockSize
	ctr = buf[keySize:]
	b = make([]byte, aes.BlockSize)
}

func parseJSONString(data []byte) (ID, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return 0, err
	}

	i, err := ParseID(s)
	if err != nil {
		return 0, err
	}

	return i, nil
}

func parseJSONInt(data []byte) (ID, error) {
	var i uint64
	if err := json.Unmarshal(data, &i); err != nil {
		return 0, err
	}

	return ID(i), nil
}
