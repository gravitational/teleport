package encryptor

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"sync"
	//"time"

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
		return nil, trace.Errorf("No values were found in the provided key")
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
		return trace.Errorf("No private key was provided in the sign key")
	}
	var err error
	e.signEntity, err = openpgp.ReadEntity(
		packet.NewReader(bytes.NewBuffer(key.PrivateValue)))
	if err != nil {
		return trace.Wrap(err)
	}
	//	e.signEntity = &(*signEntity)
	return nil
}

func (e *GPGEncryptor) AddSignCheckingKey(key Key) error {
	e.Lock()
	defer e.Unlock()

	if key.PublicValue == nil {
		return trace.Errorf("No public key was provided in the sign checking key")
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
		// converting entity to Key to check its keyID
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
	entityList := openpgp.EntityList{e.publicEntity}
	// encrypt string
	buf := new(bytes.Buffer)
	//w, err := openpgp.Encrypt(buf, entityList, nil, nil, nil)
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
	//encStr := base64.StdEncoding.EncodeToString(bytes)

	return bytes, nil
}

func (e *GPGEncryptor) Decrypt(data []byte) ([]byte, error) {
	entityList := append(openpgp.EntityList{e.privateEntity},
		e.signCheckingEntities...)

	/*entity = entityList[0]

	// Get the passphrase and read the private key.
	  // Have not touched the encrypted string yet
	  passphraseByte := []byte(passphrase)
	  log.Println("Decrypting private key using passphrase")
	  entity.PrivateKey.Decrypt(passphraseByte)
	  for _, subkey := range entity.Subkeys {
	      subkey.PrivateKey.Decrypt(passphraseByte)
	  }
	*/
	// Decode the base64 string
	/*dec, err := base64.StdEncoding.DecodeString(encString)
	  if err != nil {
	      return "", err
	  }*/

	// Decrypt it with the contents of the private key
	md, err := openpgp.ReadMessage(bytes.NewBuffer(data), entityList, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bytes, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if md.SignatureError != nil || md.Signature == nil {
		return nil, trace.Errorf("Failed to validate signature %s", md.SignatureError)
	}

	return bytes, nil
}

func GenerateGPGKey(name string) (Key, error) {
	key := Key{}
	conf := packet.Config{}
	conf.DefaultHash = crypto.SHA256

	entity, err := openpgp.NewEntity(name, "safds", "wfwq", &conf)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	/////////////////////////

	/*usrIdstring := ""
	for _, uIds := range entity.Identities {
		usrIdstring = uIds.Name

	}

	var priKey = entity.PrivateKey
	var sig = new(packet.Signature)
	//Prepare sign with our configs/////IS IT A MUST ??
	sig.Hash = crypto.SHA256
	sig.PubKeyAlgo = priKey.PubKeyAlgo
	sig.CreationTime = time.Now()
	dur := new(uint32)
	*dur = uint32(365 * 24 * 60 * 60)
	sig.SigLifetimeSecs = dur //a year
	issuerUint := new(uint64)
	*issuerUint = priKey.KeyId
	sig.IssuerKeyId = issuerUint
	sig.SigType = packet.SigTypeGenericCert

	err = sig.SignKey(entity.PrimaryKey, entity.PrivateKey, nil)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}
	err = sig.SignUserId(usrIdstring, entity.PrimaryKey, entity.PrivateKey, nil)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	entity.SignIdentity(usrIdstring, entity, nil)

	/////////*/

	/*for _, id := range entity.Identities {
		err := id.SelfSignature.SignUserId(id.UserId.Id,
			entity.PrimaryKey, entity.PrivateKey, nil)
		if err != nil {
			return Key{}, trace.Wrap(err)
		}
	}*/

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
