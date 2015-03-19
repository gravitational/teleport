/*
Package secret provides tools for encrypting and decrypting authenticated messages.
See docs/secret.md for more details.
*/
package secret

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/code.google.com/p/go.crypto/nacl/secretbox"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/random"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/metrics"
)

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

// A Service can be used to seal/open (encrypt/decrypt and authenticate) messages.
type Service struct {
	secretKey     *[SecretKeyLength]byte
	metricsClient metrics.Client
}

// New returns a new Service. Config can not be nil.
func New(config *Config) (*Service, error) {

	var err error
	var keyBytes *[SecretKeyLength]byte
	var metricsClient metrics.Client

	// read in key from keypath or if not given, try getting them from key bytes.
	if config.KeyPath != "" {
		keyBytes, err = readKeyFromDisk(config.KeyPath)
		if err != nil {
			return nil, err
		}
	} else {
		if config.KeyBytes == nil {
			return nil, fmt.Errorf("No key bytes provided.")
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

// Seal takes plaintext and returns encrypted and authenticated ciphertext.
func (s *Service) Seal(value []byte) (*SealedBytes, error) {
	return s.SealWithKey(value, s.secretKey)
}

// SealWithKey does the same thing as Seal, but a different key can be passed in.
func (s *Service) SealWithKey(value []byte, secretKey *[SecretKeyLength]byte) (*SealedBytes, error) {
	// check that we either initialized with a key or one was passed in
	if secretKey == nil {
		return nil, fmt.Errorf("secret key is nil")
	}

	// generate nonce
	nonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("unable to generate nonce: %v", err)
	}

	// use nacl secret box to encrypt plaintext
	var encrypted []byte
	encrypted = secretbox.Seal(encrypted, value, nonce, secretKey)

	// return sealed ciphertext
	return &SealedBytes{
		Ciphertext: encrypted,
		Nonce:      nonce[:],
	}, nil
}

// Open authenticates the ciphertext and if valid, decrypts and returns plaintext.
func (s *Service) Open(e *SealedBytes) ([]byte, error) {
	return s.OpenWithKey(e, s.secretKey)
}

// OpenWithKey is the same as Open, but a different key can be passed in.
func (s *Service) OpenWithKey(e *SealedBytes, secretKey *[SecretKeyLength]byte) (byt []byte, err error) {
	// once function is complete, check if we are returning err or not.
	// if we are, return emit a failure metric, if not a success metric.
	defer func() {
		if err == nil {
			s.metricsClient.Inc("success", 1, 1)
		} else {
			s.metricsClient.Inc("failure", 1, 1)
		}
	}()

	// check that we either initialized with a key or one was passed in
	if secretKey == nil {
		return nil, fmt.Errorf("secret key is nil")
	}

	// convert nonce to an array
	nonce, err := nonceSliceToArray(e.Nonce)
	if err != nil {
		return nil, err
	}

	// decrypt
	var decrypted []byte
	decrypted, ok := secretbox.Open(decrypted, e.Ciphertext, nonce, secretKey)
	if !ok {
		return nil, fmt.Errorf("unable to decrypt message")
	}

	return decrypted, nil
}

func readKeyFromDisk(keypath string) (*[SecretKeyLength]byte, error) {
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

func keySliceToArray(bytes []byte) (*[SecretKeyLength]byte, error) {
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
