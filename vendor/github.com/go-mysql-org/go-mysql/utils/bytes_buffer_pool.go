package utils

import (
	"bytes"
	"sync"
)

const (
	TooBigBlockSize = 1024 * 1024 * 4
)

var (
	bytesBufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

func BytesBufferGet() (data *bytes.Buffer) {
	data = bytesBufferPool.Get().(*bytes.Buffer)
	data.Reset()
	return data
}

func BytesBufferPut(data *bytes.Buffer) {
	if data == nil || data.Len() > TooBigBlockSize {
		return
	}
	bytesBufferPool.Put(data)
}
