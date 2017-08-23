package hotp

import "fmt"
import "testing"
import "bytes"
import "io"

var testKey = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

func newZeroHOTP() *HOTP {
	return NewHOTP(testKey, 0, 6)
}

// This test verifies that the counter increment works as expected. As per the RFC,
// the counter should be treated as an 8-byte big-endian unsigned integer.
func TestIncrement(t *testing.T) {
	otp := newZeroHOTP()
	for i := 0; i < 32; i++ {
		if otp.Counter() != uint64(i) {
			fmt.Printf("hotp: counter should be %d, is %d\n",
				i, otp.Counter())
			fmt.Printf("\tcounter state: %x\n", *otp.counter)
			t.FailNow()
		}
		otp.Increment()
	}
}

var sha1Hmac = []byte{
	0x1f, 0x86, 0x98, 0x69, 0x0e,
	0x02, 0xca, 0x16, 0x61, 0x85,
	0x50, 0xef, 0x7f, 0x19, 0xda,
	0x8e, 0x94, 0x5b, 0x55, 0x5a,
}

var truncExpect int64 = 0x50ef7f19

// This test runs through the truncation example given in the RFC.
func TestTruncate(t *testing.T) {
	if result := truncate(sha1Hmac); result != truncExpect {
		fmt.Printf("hotp: expected truncate -> %d, saw %d\n",
			truncExpect, result)
		t.FailNow()
	}

	sha1Hmac[19]++
	if result := truncate(sha1Hmac); result == truncExpect {
		fmt.Println("hotp: expected truncation to fail")
		t.FailNow()
	}
}

var rfcKey = []byte("12345678901234567890")
var rfcExpected = []string{
	"755224",
	"287082",
	"359152",
	"969429",
	"338314",
	"254676",
	"287922",
	"162583",
	"399871",
	"520489",
}

// This test runs through the test cases presented in the RFC, and
// ensures that this implementation is in compliance.
func TestRFC(t *testing.T) {
	otp := NewHOTP(rfcKey, 0, 6)
	for i := 0; i < len(rfcExpected); i++ {
		code := otp.OTP()
		if code == "" {
			fmt.Printf("hotp: failed to produce an OTP\n")
			t.FailNow()
		} else if code != rfcExpected[i] {
			fmt.Printf("hotp: invalid OTP\n")
			fmt.Printf("\tExpected: %s\n", rfcExpected[i])
			fmt.Printf("\t  Actual: %s\n", code)
			t.FailNow()
		}
	}
}

// This test uses a different key than the test cases in the RFC,
// but runs through the same test cases to ensure that they fail as
// expected.
func TestBadRFC(t *testing.T) {
	otp := NewHOTP(testKey, 0, 6)
	for i := 0; i < len(rfcExpected); i++ {
		code := otp.OTP()
		if code == "" {
			fmt.Printf("hotp: failed to produce an OTP\n")
			t.FailNow()
		} else if code == rfcExpected[i] {
			fmt.Printf("hotp: should not have received a valid OTP\n")
			t.FailNow()
		}
	}
}

// This test takes input from a test YubiKey and ensures that the
// YubiKey functionality works as expected.
func TestYubiKey(t *testing.T) {
	ykKey := []byte{
		0xd4, 0xbe, 0x97, 0xac, 0xe3,
		0x31, 0x72, 0x95, 0xd8, 0x95,
		0xeb, 0xd6, 0xb2, 0xec, 0xa6,
		0x78, 0x49, 0x79, 0x4d, 0xb3,
	}
	otp := NewHOTP(ykKey, 0, 6)
	out := []string{
		"cccc52345777705179",
		"cccc52345777404068",
		"cccc52345777490143",
		"cccc52345777739740",
		"cccc52345777043269",
		"cccc52345777035666",
		"cccc52345777725326",
	}

	codes := []string{
		"705179",
		"404068",
		"490143",
		"739740",
		"043269",
		"035666",
		"725326",
	}

	ykpub := "cccc52345777"

	if _, _, ok := otp.YubiKey("abcd"); ok {
		fmt.Println("hotp: accepted invalid YubiKey input")
		t.FailNow()
	}

	for i := 0; i < len(out); i++ {
		code := otp.OTP()
		if ykCode, pubid, ok := otp.YubiKey(out[i]); !ok {
			fmt.Printf("hotp: invalid YubiKey OTP\n")
			t.FailNow()
		} else if ykCode != code && code != codes[i] {
			fmt.Printf("hotp: YubiKey did not produce valid OTP\n")
			t.FailNow()
		} else if ykpub != pubid {
			fmt.Printf("hotp: invalid YubiKey public ID\n")
			t.FailNow()
		}
	}

	code, counter := otp.IntegrityCheck()
	if code != codes[0] {
		fmt.Println("hotp: YubiKey integrity check fails (bad code)")
		t.FailNow()
	} else if counter != uint64(len(out)) {
		fmt.Println("hotp: YubiKey integrity check fails (bad counter)")
		t.FailNow()
	}
}

// This test generates a new HOTP, outputs the URL for that HOTP,
// and attempts to parse that URL. It verifies that the two HOTPs
// are the same, and that they produce the same output.
func TestURL(t *testing.T) {
	var ident = "testuser@foo"
	otp := NewHOTP(testKey, 0, 6)
	url := otp.URL("testuser@foo")
	otp2, id, err := FromURL(url)
	if err != nil {
		fmt.Printf("hotp: failed to parse HOTP URL\n")
		t.FailNow()
	} else if id != ident {
		fmt.Printf("hotp: bad label\n")
		fmt.Printf("\texpected: %s\n", ident)
		fmt.Printf("\t  actual: %s\n", id)
		t.FailNow()
	} else if otp2.Counter() != otp.Counter() {
		fmt.Printf("hotp: OTP counters aren't synced\n")
		fmt.Printf("\toriginal: %d\n", otp.Counter())
		fmt.Printf("\t  second: %d\n", otp2.Counter())
		t.FailNow()
	}

	code1 := otp.OTP()
	code2 := otp2.OTP()
	if code1 != code2 {
		fmt.Printf("hotp: mismatched OTPs\n")
		fmt.Printf("\texpected: %s\n", code1)
		fmt.Printf("\t  actual: %s\n", code2)
	}

	// There's not much we can do test the QR code, except to
	// ensure it doesn't fail.
	_, err = otp.QR(ident)
	if err != nil {
		fmt.Printf("hotp: failed to generate QR code PNG (%v)\n", err)
		t.FailNow()
	}

	// This should fail because the maximum size of an alphanumeric
	// QR code with the lowest-level of error correction should
	// max out at 4296 bytes. 8k may be a bit overkill... but it
	// gets the job done. The value is read from the PRNG to
	// increase the likelihood that the returned data is
	// uncompressible.
	var tooBigIdent = make([]byte, 8192)
	_, err = io.ReadFull(PRNG, tooBigIdent)
	if err != nil {
		fmt.Printf("hotp: failed to read identity (%v)\n", err)
		t.FailNow()
	} else if _, err = otp.QR(string(tooBigIdent)); err == nil {
		fmt.Println("hotp: QR code should fail to encode oversized URL")
		t.FailNow()
	}
}

// This test attempts a variety of invalid urls against the parser
// to ensure they fail.
func TestBadURL(t *testing.T) {
	var urlList = []string{
		"http://google.com",
		"",
		"-",
		"foo",
		"otpauth:/foo/bar/baz",
		"://",
		"otpauth://totp/foo@bar?secret=ABCD",
		"otpauth://hotp/secret=bar",
		"otpauth://hotp/?secret=QUJDRA&algorithm=SHA256",
		"otpauth://hotp/?digits=",
		"otpauth://hotp/?secret=123",
		"otpauth://hotp/?secret=MFRGGZDF&digits=ABCD",
		"otpauth://hotp/?secret=MFRGGZDF&counter=ABCD",
	}

	for i := range urlList {
		if _, _, err := FromURL(urlList[i]); err == nil {
			fmt.Println("hotp: URL should not have parsed successfully")
			fmt.Printf("\turl was: %s\n", urlList[i])
			t.FailNow()
		}
	}
}

// This test uses a url generated with the `hotpgen` tool; this url
// was imported into the Google Authenticator app and the resulting
// codes generated by the app are checked here to verify interoperability.
func TestGAuth(t *testing.T) {
	url := "otpauth://hotp/kyle?counter=0&secret=EXZLUP7IGHQ673ZCP32RTLRU2N427Z6L"
	expected := []string{
		"023667",
		"641344",
		"419615",
		"692589",
		"237233",
		"711695",
		"620195",
	}

	otp, label, err := FromURL(url)
	if err != nil {
		fmt.Printf("hotp: failed to parse HOTP URL\n")
		t.FailNow()
	} else if label != "kyle" {
		fmt.Printf("hotp: invalid label")
		t.FailNow()
	}
	otp.Increment()

	// Validate codes
	for i := 1; i < len(expected); i++ {
		code := otp.OTP()
		if otp.Counter() != uint64(i+1) {
			fmt.Printf("hotp: invalid OTP counter (should be %d but is %d)",
				i, otp.Counter())
			t.FailNow()
		} else if code != expected[i] {
			fmt.Println("hotp: invalid OTP")
			t.FailNow()
		}
	}

	// Validate integrity check
	code, counter := otp.IntegrityCheck()
	if code != expected[0] {
		fmt.Println("hotp: invalid integrity code")
		t.FailNow()
	} else if counter != uint64(len(expected)) {
		fmt.Println("hotp: invalid integrity counter")
		t.FailNow()
	}
}

// This test verifies that a scan will successfully sync and update
// the OTP counter.
func TestScan(t *testing.T) {
	url := "otpauth://hotp/kyle?counter=0&secret=EXZLUP7IGHQ673ZCP32RTLRU2N427Z6L"
	expected := []string{
		"023667",
		"641344",
		"419615",
		"692589",
		"237233",
		"711695",
		"620195",
	}

	otp, _, err := FromURL(url)
	if err != nil {
		fmt.Printf("hotp: failed to parse HOTP URL\n")
		t.FailNow()
	}

	if !otp.Scan(expected[4], 10) {
		fmt.Println("hotp: scan should have found code")
		t.FailNow()
	}

	if otp.Counter() != 5 {
		fmt.Println("hotp: counter was not properly synced")
		t.FailNow()
	}
}

// This test verifies that a scan that does not successfully sync
// will not update the OTP counter.
func TestBadScan(t *testing.T) {
	url := "otpauth://hotp/kyle?counter=0&secret=EXZLUP7IGHQ673ZCP32RTLRU2N427Z6L"
	expected := []string{
		"023667",
		"641344",
		"419615",
		"692589",
		"237233",
		"711695",
		"620195",
	}

	otp, _, err := FromURL(url)
	if err != nil {
		fmt.Printf("hotp: failed to parse HOTP URL\n")
		t.FailNow()
	}

	if otp.Scan(expected[6], 3) {
		fmt.Println("hotp: scan should not have found code")
		t.FailNow()
	}

	if otp.Counter() != 0 {
		fmt.Println("hotp: counter was not properly synced")
		t.FailNow()
	}
}

// This test ensures that a valid code is recognised and increments
// the counter.
func TestCheck(t *testing.T) {
	otp := NewHOTP(rfcKey, 0, 6)
	if !otp.Check(rfcExpected[0]) {
		fmt.Println("hotp: Check failed when it should have succeeded")
		t.FailNow()
	}

	if otp.Counter() != 1 {
		fmt.Println("hotp: counter should have been incremented")
		t.FailNow()
	}
}

// This test ensures that an invalid code is recognised as such and
// does not increment the counter.
func TestBadCheck(t *testing.T) {
	otp := NewHOTP(rfcKey, 0, 6)
	if otp.Check(rfcExpected[1]) {
		fmt.Println("hotp: Check succeeded when it should have failed")
		t.FailNow()
	}

	if otp.Counter() != 0 {
		fmt.Println("hotp: counter should not have been incremented")
		t.FailNow()
	}
}

func TestSerialisation(t *testing.T) {
	otp := NewHOTP(rfcKey, 123456, 8)
	out, err := Marshal(otp)
	if err != nil {
		fmt.Printf("hotp: failed to marshal HOTP (%v)\n", err)
		t.FailNow()
	}

	otp2, err := Unmarshal(out)
	if err != nil {
		fmt.Printf("hotp: failed to unmarshal HOTP (%v)\n", err)
		t.FailNow()
	}

	if otp.Counter() != otp2.Counter() {
		fmt.Println("hotp: serialisation failed to preserve counter")
		t.FailNow()
	}

	if _, err = Unmarshal([]byte{0x0}); err == nil {
		fmt.Println("hotp: Unmarshal should have failed")
		t.FailNow()
	}
}

func TestGenerateHOTP(t *testing.T) {
	_, err := GenerateHOTP(6, false)
	if err != nil {
		fmt.Printf("hotp: failed to generate random key value (%v)\n", err)
		t.FailNow()
	}

	_, err = GenerateHOTP(8, true)
	if err != nil {
		fmt.Printf("hotp: failed to generate random key value (%v)\n", err)
		t.FailNow()
	}

	PRNG = new(bytes.Buffer)
	_, err = GenerateHOTP(8, true)
	if err == nil {
		fmt.Println("hotp: should have failed to generate random key value")
		t.FailNow()
	}

	// should read just enough for the key
	PRNG = bytes.NewBufferString("abcdeabcdeabcdeabcde")
	_, err = GenerateHOTP(8, true)
	if err == nil {
		fmt.Println("hotp: should have failed to generate random key value")
		t.FailNow()
	}

}

func BenchmarkFromURL(b *testing.B) {
	url := "otpauth://hotp/kyle?counter=0&secret=EXZLUP7IGHQ673ZCP32RTLRU2N427Z6L"
	for i := 0; i < b.N; i++ {
		_, _, err := FromURL(url)
		if err != nil {
			fmt.Printf("hotp: failed to parse url (%v)\n", err)
			b.FailNow()
		}
	}
}
