// package encryptedbk implements encryption layer for any backend.
package encryptedbk

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"

	"github.com/gravitational/teleport/backend"
)

type EncryptedBackend struct {
	bk          backend.Backend
	secret      secret.SecretService
	prefix      []string
	EncodingKey string
}

func newEncryptedBackend(backend backend.Backend, keyFileName string) (*EncryptedBackend, error) {
	var err error

	conf := secret.Config{
		KeyPath: keyFileName,
	}

	encryptedBk := EncryptedBackend{}
	encryptedBk.bk = backend
	encryptedBk.secret, err = secret.New(&conf)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	encryptedBk.prefix = []string{rootDir, url.QueryEscape(keyFileName)}
	encryptedBk.EncodingKey = keyFileName

	return &encryptedBk, nil
}

func (b *EncryptedBackend) IsExisting() bool {
	exists, err := b.GetVal(append(b.prefix, "exist"), "exist")
	if err != nil {
		return false
	}
	return string(exists) == "ok"
}

func (b *EncryptedBackend) SetExistence() error {
	return b.UpsertVal(append(b.prefix, "exist"),
		"exist", []byte("ok"), 0)
}

func (b *EncryptedBackend) GetKeys(path []string) ([]string, error) {
	return b.bk.GetKeys(append(b.prefix, path...))
}

func (b *EncryptedBackend) DeleteKey(path []string, key string) error {
	return b.bk.DeleteKey(append(b.prefix, path...), key)
}

func (b *EncryptedBackend) DeleteBucket(path []string, bkt string) error {
	return b.bk.DeleteBucket(append(b.prefix, path...), bkt)
}

func (b *EncryptedBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	sealedData, err := b.secret.Seal(val)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	sealedBytes := secret.SealedBytes{
		Ciphertext: sealedData.CiphertextBytes(),
		Nonce:      sealedData.NonceBytes(),
	}

	sealedBytesJSON, err := json.Marshal(sealedBytes)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	err = b.bk.UpsertVal(append(b.prefix, path...), key, sealedBytesJSON, ttl)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}
	return nil
}

func (b *EncryptedBackend) GetVal(path []string, key string) ([]byte, error) {
	sealedBytesJSON, err := b.bk.GetVal(append(b.prefix, path...), key)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	var sealedBytes secret.SealedBytes
	err = json.Unmarshal(sealedBytesJSON, &sealedBytes)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	val, err := b.secret.Open(&sealedBytes)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return val, nil
}

func (b *EncryptedBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	sealedBytesJSON, ttl, err := b.bk.GetValAndTTL(append(b.prefix, path...), key)
	if err != nil {
		log.Errorf(err.Error())
		return nil, 0, err
	}

	var sealedBytes secret.SealedBytes
	err = json.Unmarshal(sealedBytesJSON, &sealedBytes)
	if err != nil {
		log.Errorf(err.Error())
		return nil, 0, err
	}

	val, err := b.secret.Open(&sealedBytes)
	if err != nil {
		log.Errorf(err.Error())
		return nil, 0, err
	}

	return val, ttl, nil
}

const (
	rootDir = "data"
)
