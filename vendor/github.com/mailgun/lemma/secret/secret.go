/*
Package secret provides tools for encrypting and decrypting authenticated messages.
See docs/secret.md for more details.
*/
package secret

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mailgun/lemma/random"
	"github.com/mailgun/metrics"
	"golang.org/x/crypto/nacl/secretbox"
)

// SecretSevice is an interface for encrypting/decrypting and authenticating messages.
type SecretService interface {
	// Seal takes a plaintext message and returns an encrypted and authenticated ciphertext.
	Seal([]byte) (SealedData, error)

	// Open authenticates the ciphertext and, if it is valid, decrypts and returns plaintext.
	Open(SealedData) ([]byte, error)
}

// SealedData respresents an encrypted and authenticated message.
type SealedData interface {
	CiphertextBytes() []byte
	CiphertextHex() string

	NonceBytes() []byte
	NonceHex() string
}

// Config is used to configure a secret service. It contains either the key path
// or key bytes to use.
type Config struct {
	KeyPath  string
	KeyBytes *[SecretKeyLength]byte

	EmitStats    bool   // toggle emitting metrics or not
	StatsdHost   string // hostname of statsd server
	StatsdPort   int    // port of statsd server
	StatsdPrefix string // prefix to prepend to metrics
}

// SealedBytes contains the ciphertext and nonce for a sealed message.
type SealedBytes struct {
	Ciphertext []byte
	Nonce      []byte
}

func (s *SealedBytes) CiphertextBytes() []byte {
	return s.Ciphertext
}

func (s *SealedBytes) CiphertextHex() string {
	return base64.URLEncoding.EncodeToString(s.Ciphertext)
}

func (s *SealedBytes) NonceBytes() []byte {
	return s.Nonce
}

func (s *SealedBytes) NonceHex() string {
	return base64.URLEncoding.EncodeToString(s.Nonce)
}

// A Service can be used to seal/open (encrypt/decrypt and authenticate) messages.
type Service struct {
	secretKey     *[SecretKeyLength]byte
	metricsClient metrics.Client
}

// New returns a new Service. Config can not be nil.
func New(config *Config) (SecretService, error) {
	var err error
	var keyBytes *[SecretKeyLength]byte
	var metricsClient metrics.Client

	// Read in key from KeyPath or if not given, try getting them from KeyBytes.
	if config.KeyPath != "" {
		if keyBytes, err = ReadKeyFromDisk(config.KeyPath); err != nil {
			return nil, err
		}
	} else {
		if config.KeyBytes == nil {
			return nil, errors.New("No key bytes provided.")
		}
		keyBytes = config.KeyBytes
	}

	// setup metrics service
	if config.EmitStats {
		// get hostname of box
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain hostname: %v", err)
		}

		// build lemma prefix
		prefix := "lemma." + strings.Replace(hostname, ".", "_", -1)
		if config.StatsdPrefix != "" {
			prefix += "." + config.StatsdPrefix
		}

		// build metrics client
		hostport := fmt.Sprintf("%v:%v", config.StatsdHost, config.StatsdPort)
		metricsClient, err = metrics.NewWithOptions(hostport, prefix, metrics.Options{UseBuffering: true})
		if err != nil {
			return nil, err
		}
	} else {
		// if you don't want to emit stats, use the nop client
		metricsClient = metrics.NewNop()
	}

	return &Service{
		secretKey:     keyBytes,
		metricsClient: metricsClient,
	}, nil
}

// Seal takes plaintext and a key and returns encrypted and authenticated ciphertext.
// Allows passing in a key and useful for one off sealing purposes, otherwise
// create a secret.Service to seal multiple times.
func Seal(value []byte, secretKey *[SecretKeyLength]byte) (SealedData, error) {
	if secretKey == nil {
		return nil, fmt.Errorf("secret key is nil")
	}

	secretService, err := New(&Config{KeyBytes: secretKey})
	if err != nil {
		return nil, err
	}

	return secretService.Seal(value)
}

// Open authenticates the ciphertext and if valid, decrypts and returns plaintext.
// Allows passing in a key and useful for one off opening purposes, otherwise
// create a secret.Service to open multiple times.
func Open(e SealedData, secretKey *[SecretKeyLength]byte) ([]byte, error) {
	if secretKey == nil {
		return nil, fmt.Errorf("secret key is nil")
	}

	secretService, err := New(&Config{KeyBytes: secretKey})
	if err != nil {
		return nil, err
	}

	return secretService.Open(e)
}

// Seal takes plaintext and returns encrypted and authenticated ciphertext.
func (s *Service) Seal(value []byte) (SealedData, error) {
	// generate nonce
	nonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("unable to generate nonce: %v", err)
	}

	// use nacl secret box to encrypt plaintext
	var encrypted []byte
	encrypted = secretbox.Seal(encrypted, value, nonce, s.secretKey)

	// return sealed ciphertext
	return &SealedBytes{
		Ciphertext: encrypted,
		Nonce:      nonce[:],
	}, nil
}

// Open authenticates the ciphertext and if valid, decrypts and returns plaintext.
func (s *Service) Open(e SealedData) (byt []byte, err error) {
	// once function is complete, check if we are returning err or not.
	// if we are, return emit a failure metric, if not a success metric.
	defer func() {
		if err == nil {
			s.metricsClient.Inc("success", 1, 1)
		} else {
			s.metricsClient.Inc("failure", 1, 1)
		}
	}()

	// convert nonce to an array
	nonce, err := nonceSliceToArray(e.NonceBytes())
	if err != nil {
		return nil, err
	}

	// decrypt
	var decrypted []byte
	decrypted, ok := secretbox.Open(decrypted, e.CiphertextBytes(), nonce, s.secretKey)
	if !ok {
		return nil, fmt.Errorf("unable to decrypt message")
	}

	return decrypted, nil
}

func ReadKeyFromDisk(keypath string) (*[SecretKeyLength]byte, error) {
	// load key from disk
	keyBytes, err := ioutil.ReadFile(keypath)
	if err != nil {
		return nil, err
	}

	// strip newline (\n or 0x0a) if it's at the end
	keyBytes = bytes.TrimSuffix(keyBytes, []byte("\n"))

	// decode string and convert to array and return it
	return EncodedStringToKey(string(keyBytes))
}

func KeySliceToArray(bytes []byte) (*[SecretKeyLength]byte, error) {
	// check that the lengths match
	if len(bytes) != SecretKeyLength {
		return nil, fmt.Errorf("wrong key length: %v", len(bytes))
	}

	// copy bytes into array
	var keyBytes [SecretKeyLength]byte
	copy(keyBytes[:], bytes)

	return &keyBytes, nil
}

func nonceSliceToArray(bytes []byte) (*[NonceLength]byte, error) {
	// check that the lengths match
	if len(bytes) != NonceLength {
		return nil, fmt.Errorf("wrong nonce length: %v", len(bytes))
	}

	// copy bytes into array
	var nonceBytes [NonceLength]byte
	copy(nonceBytes[:], bytes)

	return &nonceBytes, nil
}

func generateNonce() (*[NonceLength]byte, error) {
	// get b-bytes of random from /dev/urandom
	bytes, err := randomProvider.Bytes(NonceLength)
	if err != nil {
		return nil, err
	}

	return nonceSliceToArray(bytes)
}

var randomProvider random.RandomProvider

// init sets the package level randomProvider to be a real csprng. this is done
// so during tests, we can use a fake random number generator.
func init() {
	randomProvider = &random.CSPRNG{}
}
