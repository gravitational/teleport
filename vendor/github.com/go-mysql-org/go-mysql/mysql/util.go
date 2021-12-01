package mysql

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/pingcap/errors"
	"github.com/siddontang/go/hack"
)

func Pstack() string {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	return string(buf[0:n])
}

func CalcPassword(scramble, password []byte) []byte {
	if len(password) == 0 {
		return nil
	}

	// stage1Hash = SHA1(password)
	crypt := sha1.New()
	crypt.Write(password)
	stage1 := crypt.Sum(nil)

	// scrambleHash = SHA1(scramble + SHA1(stage1Hash))
	// inner Hash
	crypt.Reset()
	crypt.Write(stage1)
	hash := crypt.Sum(nil)

	// outer Hash
	crypt.Reset()
	crypt.Write(scramble)
	crypt.Write(hash)
	scramble = crypt.Sum(nil)

	// token = scrambleHash XOR stage1Hash
	for i := range scramble {
		scramble[i] ^= stage1[i]
	}
	return scramble
}

// CalcCachingSha2Password: Hash password using MySQL 8+ method (SHA256)
func CalcCachingSha2Password(scramble []byte, password string) []byte {
	if len(password) == 0 {
		return nil
	}

	// XOR(SHA256(password), SHA256(SHA256(SHA256(password)), scramble))

	crypt := sha256.New()
	crypt.Write([]byte(password))
	message1 := crypt.Sum(nil)

	crypt.Reset()
	crypt.Write(message1)
	message1Hash := crypt.Sum(nil)

	crypt.Reset()
	crypt.Write(message1Hash)
	crypt.Write(scramble)
	message2 := crypt.Sum(nil)

	for i := range message1 {
		message1[i] ^= message2[i]
	}

	return message1
}

func EncryptPassword(password string, seed []byte, pub *rsa.PublicKey) ([]byte, error) {
	plain := make([]byte, len(password)+1)
	copy(plain, password)
	for i := range plain {
		j := i % len(seed)
		plain[i] ^= seed[j]
	}
	sha1v := sha1.New()
	return rsa.EncryptOAEP(sha1v, rand.Reader, pub, plain, nil)
}

// AppendLengthEncodedInteger: encodes a uint64 value and appends it to the given bytes slice
func AppendLengthEncodedInteger(b []byte, n uint64) []byte {
	switch {
	case n <= 250:
		return append(b, byte(n))

	case n <= 0xffff:
		return append(b, 0xfc, byte(n), byte(n>>8))

	case n <= 0xffffff:
		return append(b, 0xfd, byte(n), byte(n>>8), byte(n>>16))
	}
	return append(b, 0xfe, byte(n), byte(n>>8), byte(n>>16), byte(n>>24),
		byte(n>>32), byte(n>>40), byte(n>>48), byte(n>>56))
}

func RandomBuf(size int) ([]byte, error) {
	buf := make([]byte, size)

	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return nil, errors.Trace(err)
	}

	// avoid to generate '\0'
	for i, b := range buf {
		if b == 0 {
			buf[i] = '0'
		}
	}

	return buf, nil
}

// FixedLengthInt: little endian
func FixedLengthInt(buf []byte) uint64 {
	var num uint64 = 0
	for i, b := range buf {
		num |= uint64(b) << (uint(i) * 8)
	}
	return num
}

// BFixedLengthInt: big endian
func BFixedLengthInt(buf []byte) uint64 {
	var num uint64 = 0
	for i, b := range buf {
		num |= uint64(b) << (uint(len(buf)-i-1) * 8)
	}
	return num
}

func LengthEncodedInt(b []byte) (num uint64, isNull bool, n int) {
	if len(b) == 0 {
		return 0, true, 1
	}

	switch b[0] {
	// 251: NULL
	case 0xfb:
		return 0, true, 1

		// 252: value of following 2
	case 0xfc:
		return uint64(b[1]) | uint64(b[2])<<8, false, 3

		// 253: value of following 3
	case 0xfd:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16, false, 4

		// 254: value of following 8
	case 0xfe:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 |
				uint64(b[4])<<24 | uint64(b[5])<<32 | uint64(b[6])<<40 |
				uint64(b[7])<<48 | uint64(b[8])<<56,
			false, 9
	}

	// 0-250: value of first byte
	return uint64(b[0]), false, 1
}

func PutLengthEncodedInt(n uint64) []byte {
	switch {
	case n <= 250:
		return []byte{byte(n)}

	case n <= 0xffff:
		return []byte{0xfc, byte(n), byte(n >> 8)}

	case n <= 0xffffff:
		return []byte{0xfd, byte(n), byte(n >> 8), byte(n >> 16)}

	case n <= 0xffffffffffffffff:
		return []byte{0xfe, byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24),
			byte(n >> 32), byte(n >> 40), byte(n >> 48), byte(n >> 56)}
	}
	return nil
}

// LengthEncodedString returns the string read as a bytes slice, whether the value is NULL,
// the number of bytes read and an error, in case the string is longer than
// the input slice
func LengthEncodedString(b []byte) ([]byte, bool, int, error) {
	// Get length
	num, isNull, n := LengthEncodedInt(b)
	if num < 1 {
		return b[n:n], isNull, n, nil
	}

	n += int(num)

	// Check data length
	if len(b) >= n {
		return b[n-int(num) : n : n], false, n, nil
	}
	return nil, false, n, io.EOF
}

func SkipLengthEncodedString(b []byte) (int, error) {
	// Get length
	num, _, n := LengthEncodedInt(b)
	if num < 1 {
		return n, nil
	}

	n += int(num)

	// Check data length
	if len(b) >= n {
		return n, nil
	}
	return n, io.EOF
}

func PutLengthEncodedString(b []byte) []byte {
	data := make([]byte, 0, len(b)+9)
	data = append(data, PutLengthEncodedInt(uint64(len(b)))...)
	data = append(data, b...)
	return data
}

func Uint16ToBytes(n uint16) []byte {
	return []byte{
		byte(n),
		byte(n >> 8),
	}
}

func Uint32ToBytes(n uint32) []byte {
	return []byte{
		byte(n),
		byte(n >> 8),
		byte(n >> 16),
		byte(n >> 24),
	}
}

func Uint64ToBytes(n uint64) []byte {
	return []byte{
		byte(n),
		byte(n >> 8),
		byte(n >> 16),
		byte(n >> 24),
		byte(n >> 32),
		byte(n >> 40),
		byte(n >> 48),
		byte(n >> 56),
	}
}

func FormatBinaryDate(n int, data []byte) ([]byte, error) {
	switch n {
	case 0:
		return []byte("0000-00-00"), nil
	case 4:
		return []byte(fmt.Sprintf("%04d-%02d-%02d",
			binary.LittleEndian.Uint16(data[:2]),
			data[2],
			data[3])), nil
	default:
		return nil, errors.Errorf("invalid date packet length %d", n)
	}
}

func FormatBinaryDateTime(n int, data []byte) ([]byte, error) {
	switch n {
	case 0:
		return []byte("0000-00-00 00:00:00"), nil
	case 4:
		return []byte(fmt.Sprintf("%04d-%02d-%02d 00:00:00",
			binary.LittleEndian.Uint16(data[:2]),
			data[2],
			data[3])), nil
	case 7:
		return []byte(fmt.Sprintf(
			"%04d-%02d-%02d %02d:%02d:%02d",
			binary.LittleEndian.Uint16(data[:2]),
			data[2],
			data[3],
			data[4],
			data[5],
			data[6])), nil
	case 11:
		return []byte(fmt.Sprintf(
			"%04d-%02d-%02d %02d:%02d:%02d.%06d",
			binary.LittleEndian.Uint16(data[:2]),
			data[2],
			data[3],
			data[4],
			data[5],
			data[6],
			binary.LittleEndian.Uint32(data[7:11]))), nil
	default:
		return nil, errors.Errorf("invalid datetime packet length %d", n)
	}
}

func FormatBinaryTime(n int, data []byte) ([]byte, error) {
	if n == 0 {
		return []byte("0000-00-00"), nil
	}

	var sign byte
	if data[0] == 1 {
		sign = byte('-')
	}

	switch n {
	case 8:
		return []byte(fmt.Sprintf(
			"%c%02d:%02d:%02d",
			sign,
			uint16(data[1])*24+uint16(data[5]),
			data[6],
			data[7],
		)), nil
	case 12:
		return []byte(fmt.Sprintf(
			"%c%02d:%02d:%02d.%06d",
			sign,
			uint16(data[1])*24+uint16(data[5]),
			data[6],
			data[7],
			binary.LittleEndian.Uint32(data[8:12]),
		)), nil
	default:
		return nil, errors.Errorf("invalid time packet length %d", n)
	}
}

var (
	DONTESCAPE = byte(255)

	EncodeMap [256]byte
)

// Escape: only support utf-8
func Escape(sql string) string {
	dest := make([]byte, 0, 2*len(sql))

	for _, w := range hack.Slice(sql) {
		if c := EncodeMap[w]; c == DONTESCAPE {
			dest = append(dest, w)
		} else {
			dest = append(dest, '\\', c)
		}
	}

	return string(dest)
}

func GetNetProto(addr string) string {
	if strings.Contains(addr, "/") {
		return "unix"
	} else {
		return "tcp"
	}
}

// ErrorEqual returns a boolean indicating whether err1 is equal to err2.
func ErrorEqual(err1, err2 error) bool {
	e1 := errors.Cause(err1)
	e2 := errors.Cause(err2)

	if e1 == e2 {
		return true
	}

	if e1 == nil || e2 == nil {
		return e1 == e2
	}

	return e1.Error() == e2.Error()
}

var encodeRef = map[byte]byte{
	'\x00': '0',
	'\'':   '\'',
	'"':    '"',
	'\b':   'b',
	'\n':   'n',
	'\r':   'r',
	'\t':   't',
	26:     'Z', // ctl-Z
	'\\':   '\\',
}

func init() {
	for i := range EncodeMap {
		EncodeMap[i] = DONTESCAPE
	}
	for i := range EncodeMap {
		if to, ok := encodeRef[byte(i)]; ok {
			EncodeMap[byte(i)] = to
		}
	}
}
