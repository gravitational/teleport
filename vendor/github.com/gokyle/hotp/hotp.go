package hotp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/asn1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/url"
	"strconv"

	"github.com/mdp/rsc/qr"
)

// RFC 4226 specifies the counter as being 8 bytes.
const ctrSize = 8

// ErrInvalidHOTPURL is returned via FromURL; it indicates a malformed
// HOTP otpauth URL.
var ErrInvalidHOTPURL = errors.New("hotp: invalid HOTP url")

// PRNG is the source of random data; this is used by GenerateHOTP
// and should be a cryptographically-secure PRNG.
var PRNG = rand.Reader

// HOTP represents a new key value for generating one-time passwords;
// it contains the key used to construct one-time passwords and the
// counter state used in the OTP generation. Digits contains the
// number of digits that generated OTPs should output. Key is a
// cryptographic secret, and should be treated as such.
type HOTP struct {
	Key     []byte
	counter *[ctrSize]byte
	Digits  int
}

// Counter returns the HOTP's 8-byte counter as an unsigned 64-bit
// integer.
func (otp HOTP) Counter() uint64 {
	counter := binary.BigEndian.Uint64(otp.counter[:])
	return counter
}

// Increment will increment an HOTP source's counter. This is useful
// for providers like the Google Authenticator app, which immediately
// increments the counter and uses the 0 counter value as an integrity
// check.
func (otp HOTP) Increment() {
	for i := ctrSize - 1; i >= 0; i-- {
		if otp.counter[i]++; otp.counter[i] != 0 {
			return
		}
	}
}

// OTP generates a new one-time password.
func (otp HOTP) OTP() string {
	h := hmac.New(sha1.New, otp.Key)
	h.Write(otp.counter[:])
	otp.Increment()
	hash := h.Sum(nil)
	result := truncate(hash)

	mod := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(otp.Digits)), nil)
	mod = mod.Mod(big.NewInt(result), mod)
	fmtStr := fmt.Sprintf("%%0%dd", otp.Digits)
	return fmt.Sprintf(fmtStr, mod.Uint64())
}

func (otp *HOTP) setCounter(counter uint64) bool {
	if otp.counter == nil {
		otp.counter = new([ctrSize]byte)
	}
	binary.BigEndian.PutUint64(otp.counter[:], counter)
	return true
}

// NewHOTP intialises a new HOTP instance with the key and counter
// values. No check is done on the digits, but typical values are 6
// and 8.
func NewHOTP(key []byte, counter uint64, digits int) *HOTP {
	otp := &HOTP{
		Key:    key,
		Digits: digits,
	}
	otp.counter = new([ctrSize]byte)
	binary.BigEndian.PutUint64(otp.counter[:], counter)

	return otp
}

// URL returns a suitable URL, such as for the Google Authenticator
// app. The label is used by these apps to identify the service to
// which this OTP belongs. The digits value is ignored by the Google
// authenticator app, and is therefore elided in the resulting URL.
func (otp *HOTP) URL(label string) string {
	secret := base32.StdEncoding.EncodeToString(otp.Key)
	u := url.URL{}
	v := url.Values{}
	u.Scheme = "otpauth"
	u.Host = "hotp"
	u.Path = label
	v.Add("secret", secret)
	v.Add("counter", fmt.Sprintf("%d", otp.Counter()))
	u.RawQuery = v.Encode()
	return u.String()
}

// QR generates a byte slice containing the a QR code encoded as a
// PNG with level Q error correction.
func (otp *HOTP) QR(label string) ([]byte, error) {
	u := otp.URL(label)
	code, err := qr.Encode(u, qr.Q)
	if err != nil {
		return nil, err
	}
	return code.PNG(), nil
}

// truncate contains the DT function from the RFC; this is used to
// deterministically select a sequence of 4 bytes from the HMAC
// counter hash.
func truncate(in []byte) int64 {
	offset := int(in[len(in)-1] & 0xF)
	p := in[offset : offset+4]
	var binCode int32
	binCode = int32((p[0] & 0x7f)) << 24
	binCode += int32((p[1] & 0xff)) << 16
	binCode += int32((p[2] & 0xff)) << 8
	binCode += int32((p[3] & 0xff))
	return int64(binCode) & 0x7FFFFFFF
}

// FromURL parses a new HOTP from a URL string. It returns the OTP,
// the label associated with the OTP, and any errors that occurred.
func FromURL(urlString string) (*HOTP, string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, "", err
	}

	if u.Scheme != "otpauth" {
		return nil, "", ErrInvalidHOTPURL
	} else if u.Host != "hotp" {
		return nil, "", ErrInvalidHOTPURL
	}

	v := u.Query()
	if len(v) == 0 {
		return nil, "", ErrInvalidHOTPURL
	}
	if v.Get("secret") == "" {
		return nil, "", ErrInvalidHOTPURL
	} else if algo := v.Get("algorithm"); algo != "" && algo != "SHA1" {
		return nil, "", ErrInvalidHOTPURL
	}

	var identity string
	if len(u.Path) > 1 {
		identity = u.Path[1:]
	}

	var counter uint64
	if ctr := v.Get("counter"); ctr != "" {
		counter, err = strconv.ParseUint(ctr, 10, 64)
		if err != nil {
			return nil, "", ErrInvalidHOTPURL
		}
	}

	secret, err := base32.StdEncoding.DecodeString(v.Get("secret"))
	if err != nil {
		return nil, "", ErrInvalidHOTPURL
	}

	var digits int64 = 6
	if v.Get("digits") != "" {
		digits, err = strconv.ParseInt(v.Get("digits"), 10, 8)
		if err != nil {
			return nil, "", ErrInvalidHOTPURL
		}
	}

	otp := NewHOTP(secret, counter, int(digits))
	return otp, identity, nil
}

// GenerateHOTP will generate a randomised HOTP source; if the
// randCounter parameter is true, the counter will be randomised.
func GenerateHOTP(digits int, randCounter bool) (*HOTP, error) {
	key := make([]byte, sha1.Size)
	_, err := io.ReadFull(PRNG, key)
	if err != nil {
		return nil, err
	}

	var counter uint64
	if randCounter {
		ctr, err := rand.Int(PRNG, big.NewInt(int64(math.MaxInt64)))
		if err != nil {
			return nil, err
		}
		counter = ctr.Uint64()
	}

	return NewHOTP(key, counter, digits), nil
}

// YubiKey reads an OATH-HOTP string as returned by a YubiKey, and
// returns three values. The first value contains the actual OTP, the
// second value contains the YubiKey's token identifier, and the final
// value indicates whether the input string was a valid YubiKey
// OTP. This does not check whether the code is correct or not, it
// only ensures that it is well-formed output from a token and
// splits the output into the code and the public identity.
func (otp *HOTP) YubiKey(in string) (string, string, bool) {
	if len(in) < otp.Digits {
		return "", "", false
	}

	otpStart := len(in) - otp.Digits
	code := in[otpStart:]
	pubid := in[:otpStart]
	return code, pubid, true
}

// IntegrityCheck returns two values, the base OTP and the current
// counter. This is used, for example, with the Google Authenticator
// app's "Check key value" function and can be used to verify that
// the application and the provider are in sync.
func (otp *HOTP) IntegrityCheck() (string, uint64) {
	h := hmac.New(sha1.New, otp.Key)
	counter := make([]byte, 8)
	h.Write(counter)
	hash := h.Sum(nil)
	result := truncate(hash)

	mod := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(otp.Digits)), nil)
	mod = mod.Mod(big.NewInt(result), mod)
	fmtStr := fmt.Sprintf("%%0%dd", otp.Digits)
	return fmt.Sprintf(fmtStr, mod.Uint64()), otp.Counter()
}

// Scan takes a code input (i.e. from the user), and scans ahead
// within a certain window of counter values. This can be used in the
// case where the server's counter and the user's counter have fallen
// out of sync.
func (otp *HOTP) Scan(code string, window int) bool {
	var valid bool
	codeBytes := []byte(code)
	counter := otp.Counter()

	for i := 0; i < window; i++ {
		genCode := []byte(otp.OTP())
		if subtle.ConstantTimeCompare(codeBytes, genCode) == 1 {
			valid = true
			break
		}
	}
	if !valid {
		otp.setCounter(counter)
	}
	return valid
}

// Check takes an input code and verifies it against the OTP. If
// successful, the counter is incremented.
func (otp *HOTP) Check(code string) bool {
	codeBytes := []byte(code)
	genCode := []byte(otp.OTP())
	if subtle.ConstantTimeCompare(codeBytes, genCode) != 1 {
		otp.setCounter(otp.Counter() - 1)
		return false
	}
	return true
}

// Marshal serialises an HOTP key value as a DER-encoded byte slice.
func Marshal(otp *HOTP) ([]byte, error) {
	var asnHOTP struct {
		Key     []byte
		Counter *big.Int
		Digits  int
	}
	asnHOTP.Key = otp.Key[:]
	asnHOTP.Counter = new(big.Int).SetUint64(otp.Counter())
	asnHOTP.Digits = otp.Digits
	return asn1.Marshal(asnHOTP)
}

// Unmarshal parses a DER-encoded serialised HOTP key value.
func Unmarshal(in []byte) (otp *HOTP, err error) {
	var asnHOTP struct {
		Key     []byte
		Counter *big.Int
		Digits  int
	}
	_, err = asn1.Unmarshal(in, &asnHOTP)
	if err != nil {
		return
	}

	otp = &HOTP{
		Key:    asnHOTP.Key[:],
		Digits: asnHOTP.Digits,
	}
	otp.setCounter(asnHOTP.Counter.Uint64())
	return
}
