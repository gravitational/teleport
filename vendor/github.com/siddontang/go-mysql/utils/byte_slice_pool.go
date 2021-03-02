package utils

import "sync"

var (
	byteSlicePool = sync.Pool{
		New: func() interface{} {
			return []byte{}
		},
	}
	byteSliceChan = make(chan []byte, 10)
)

func ByteSliceGet(length int) (data []byte) {
	select {
	case data = <-byteSliceChan:
	default:
		data = byteSlicePool.Get().([]byte)[:0]
	}

	if cap(data) < length {
		data = make([]byte, length)
	} else {
		data = data[:length]
	}

	return data
}

func ByteSlicePut(data []byte) {
	select {
	case byteSliceChan <- data:
	default:
		byteSlicePool.Put(data)
	}
}
