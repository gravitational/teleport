package client

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
)

const (
	defaultKeyDir = ".tsh"
	sessionKeyDir = "sessions"
	fileNameCert  = "cert"
	fileNameKey   = "key"
	fileNamePub   = "pub"
	fileNameTTL   = ".ttl"
)

// FSLocalKeyStore implements LocalKeyStore interface using the filesystem
type FSLocalKeyStore struct {
	LocalKeyStore

	// KeyDir is the directory where all keys are stored
	KeyDir string
}

// NewFSLocalKeyStore creates a new filesystem-based local keystore object
// and initializes it.
//
// if dirPath is empty, sets it to ~/.tsh
func NewFSLocalKeyStore(dirPath string) (s *FSLocalKeyStore, err error) {
	log.Infof("using FSLocalKeyStore")
	dirPath, err = initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FSLocalKeyStore{
		KeyDir: dirPath,
	}, nil
}

func (fs *FSLocalKeyStore) GetKeys() (keys []Key, err error) {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir)
	if !isDir(dirPath) {
		return make([]Key, 0), nil
	}
	dirEntries, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, fi := range dirEntries {
		if !fi.IsDir() {
			continue
		}
		k, err := fs.GetKey(fi.Name())
		if err != nil {
			// if a key is reported as 'not found' it's probably because it expired
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			} else {
				continue
			}
		}
		keys = append(keys, *k)
	}
	return keys, nil
}

func (fs *FSLocalKeyStore) AddKey(host string, key *Key) error {
	dirPath, err := fs.dirFor(host)
	if err != nil {
		return trace.Wrap(err)
	}
	writeBytes := func(fname string, data []byte) error {
		fp := filepath.Join(dirPath, fname)
		err := ioutil.WriteFile(fp, data, 0640)
		if err != nil {
			log.Error(err)
		}
		return err
	}
	if err = writeBytes(fileNameCert, key.Cert); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(fileNamePub, key.Pub); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(fileNameKey, key.Priv); err != nil {
		return trace.Wrap(err)
	}
	ttl, _ := key.Deadline.MarshalJSON()
	if err = writeBytes(fileNameTTL, ttl); err != nil {
		return trace.Wrap(err)
	}
	log.Infof("keystore.AddKey(%s)", host)
	return nil
}

func (fs *FSLocalKeyStore) GetKey(host string) (*Key, error) {
	dirPath, err := fs.dirFor(host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ttl, err := ioutil.ReadFile(filepath.Join(dirPath, fileNameTTL))
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	var deadline time.Time
	if err = deadline.UnmarshalJSON(ttl); err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	// this session key is expired
	if deadline.Before(time.Now()) {
		os.RemoveAll(dirPath)
		return nil, trace.NotFound("session keys for %s are not found", host)
	}
	cert, err := ioutil.ReadFile(filepath.Join(dirPath, fileNameCert))
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	pub, err := ioutil.ReadFile(filepath.Join(dirPath, fileNamePub))
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	priv, err := ioutil.ReadFile(filepath.Join(dirPath, fileNameKey))
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	log.Infof("keystore.Get(%v)", host)
	return &Key{
		Pub:      pub,
		Priv:     priv,
		Cert:     cert,
		Deadline: deadline,
	}, nil
}

func (fs *FSLocalKeyStore) AddKnownHost(hostname string, publicKeys []ssh.PublicKey) error {
	return nil
}

func (fs *FSLocalKeyStore) dirFor(hostname string) (string, error) {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir, hostname)
	if !isDir(dirPath) {
		if err := os.MkdirAll(dirPath, 0777); err != nil {
			log.Error(err)
			return "", trace.Wrap(err)
		}
	}
	return dirPath, nil
}

func initKeysDir(dirPath string) (string, error) {
	var err error
	// not specified? use `~/.tsh`
	if dirPath == "" {
		u, err := user.Current()
		if err != nil {
			dirPath = os.TempDir()
		} else {
			dirPath = u.HomeDir
		}
		dirPath = filepath.Join(dirPath, defaultKeyDir)
	}
	// create if doesn't exist:
	_, err = os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, os.ModeDir|0777)
			if err != nil {
				return "", trace.Wrap(err)
			}
		} else {
			return "", trace.Wrap(err)
		}
	}

	return dirPath, nil
}

func isDir(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err == nil {
		return fi.IsDir()
	}
	return false
}
