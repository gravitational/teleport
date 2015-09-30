package encryptor

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"sync"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"

	_ "golang.org/x/crypto/ripemd160"
)

type GPGEncryptor struct {
	publicEntity         *openpgp.Entity
	privateEntity        *openpgp.Entity
	signEntity           *openpgp.Entity
	signCheckingEntities openpgp.EntityList
	*sync.Mutex
}

func NewGPGEncryptor(key Key) (*GPGEncryptor, error) {
	e := GPGEncryptor{}
	e.Mutex = &sync.Mutex{}

	if key.PublicValue == nil && key.PrivateValue == nil {
		return nil, trace.Errorf("no values were found in the provided key")
	}

	if key.PublicValue != nil {
		var err error
		e.publicEntity, err = openpgp.ReadEntity(
			packet.NewReader(bytes.NewBuffer(key.PublicValue)))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if key.PrivateValue != nil {
		var err error
		e.privateEntity, err = openpgp.ReadEntity(
			packet.NewReader(bytes.NewBuffer(key.PrivateValue)))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &e, nil
}

func (e *GPGEncryptor) SetSignKey(key Key) error {
	e.Lock()
	defer e.Unlock()

	if key.PrivateValue == nil {
		return trace.Errorf("no private key was provided in the sign key")
	}
	var err error
	e.signEntity, err = openpgp.ReadEntity(
		packet.NewReader(bytes.NewBuffer(key.PrivateValue)))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (e *GPGEncryptor) AddSignCheckingKey(key Key) error {
	e.Lock()
	defer e.Unlock()

	if key.PublicValue == nil {
		return trace.Errorf("no public key was provided in the sign checking key")
	}
	signCheckingEntity, err := openpgp.ReadEntity(
		packet.NewReader(bytes.NewBuffer(key.PublicValue)))
	if err != nil {
		return trace.Wrap(err)
	}
	e.signCheckingEntities = append(e.signCheckingEntities, signCheckingEntity)
	return nil
}

func (e *GPGEncryptor) DeleteSignCheckingKey(keyID string) error {
	e.Lock()
	defer e.Unlock()

	selectedEntities := []int{}

	for i, entity := range e.signCheckingEntities {
		bufPub := new(bytes.Buffer)
		if err := entity.Serialize(bufPub); err != nil {
			return trace.Wrap(err)
		}
		publicValue, err := ioutil.ReadAll(bufPub)
		if err != nil {
			return trace.Wrap(err)
		}

		keyIDSha := sha256.Sum256(publicValue[:])
		curKeyid := hex.EncodeToString(keyIDSha[:])

		if keyID == curKeyid {
			selectedEntities = append(selectedEntities, i)
		}
	}

	for i := len(selectedEntities) - 1; i >= 0; i-- {
		x := selectedEntities[i]
		e.signCheckingEntities = append(e.signCheckingEntities[:x],
			e.signCheckingEntities[x+1:]...)
	}

	return nil
}

func (e *GPGEncryptor) Encrypt(data []byte) ([]byte, error) {
	if e.publicEntity == nil {
		return nil, trace.Errorf("used key doesn't have public value to encrypt")
	}
	entityList := openpgp.EntityList{e.publicEntity}
	// encrypt string
	buf := new(bytes.Buffer)
	w, err := openpgp.Encrypt(buf, entityList, e.signEntity, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = w.Write(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = w.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bytes, err := ioutil.ReadAll(buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hexString := hex.EncodeToString(bytes)

	return []byte(hexString), nil
}

func (e *GPGEncryptor) Decrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, trace.Errorf("Decryption error: empty input data")
	}

	if e.privateEntity == nil {
		return nil, trace.Errorf("used key doesn't have private value to decrypt")
	}
	entityList := append(openpgp.EntityList{e.privateEntity},
		e.signCheckingEntities...)

	hexString := string(data)
	encryptedBytes, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Decrypt it with the contents of the private key
	md, err := openpgp.ReadMessage(bytes.NewBuffer(encryptedBytes), entityList, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bytes, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if md.SignatureError != nil || md.Signature == nil {
		return nil, trace.Errorf("failed to validate signature %s", md.SignatureError)
	}

	return bytes, nil
}

func GenerateGPGKey(name string) (Key, error) {
	key := Key{}
	conf := packet.Config{}
	conf.DefaultHash = crypto.SHA256

	entity, err := openpgp.NewEntity(name, "", "", &conf)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	bufPriv := new(bytes.Buffer)
	if err := entity.SerializePrivate(bufPriv, nil); err != nil {
		return Key{}, trace.Wrap(err)
	}
	key.PrivateValue, err = ioutil.ReadAll(bufPriv)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	bufPub := new(bytes.Buffer)
	if err := entity.Serialize(bufPub); err != nil {
		return Key{}, trace.Wrap(err)
	}
	key.PublicValue, err = ioutil.ReadAll(bufPub)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	keyIDSha := sha256.Sum256(key.PublicValue[:])
	keyID := hex.EncodeToString(keyIDSha[:])

	key.ID = keyID
	key.Name = name
	return key, nil
}
