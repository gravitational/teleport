package utils

import (
	"encoding/json"
	"io/ioutil"

	"github.com/gravitational/trace"
)

// AddrStorage is used to store information locally for
// every client that connects in the cluster, so it can always have
// up-to-date info about auth servers
type AddrStorage interface {
	// SetAddresses saves addresses
	SetAddresses([]NetAddr) error
	// GetAddresses
	GetAddresses() ([]NetAddr, error)
}

// FileAddrStorage is a file based address storage
type FileAddrStorage struct {
	filePath string
}

// SetAddresses updates storage with new address list
func (fs *FileAddrStorage) SetAddresses(addrs []NetAddr) error {
	bytes, err := json.Marshal(addrs)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(fs.filePath, bytes, 0666)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// GetAddresses returns saved address list
func (fs *FileAddrStorage) GetAddresses() ([]NetAddr, error) {
	bytes, err := ioutil.ReadFile(fs.filePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var addrs []NetAddr
	err = json.Unmarshal(bytes, &addrs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return addrs, nil
}

// NewFileAddrStorage returns new instance of file-based address storage
func NewFileAddrStorage(filePath string) *FileAddrStorage {
	return &FileAddrStorage{
		filePath: filePath,
	}
}
