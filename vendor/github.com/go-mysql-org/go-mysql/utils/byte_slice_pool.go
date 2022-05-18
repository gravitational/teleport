package utils

import "sync"

type ByteSlice struct {
	B []byte
}

var (
	byteSlicePool = sync.Pool{
		New: func() interface{} {
			return new(ByteSlice)
		},
	}
)

func ByteSliceGet(length int) *ByteSlice {
	data := byteSlicePool.Get().(*ByteSlice)
	if cap(data.B) < length {
		data.B = make([]byte, length)
	} else {
		data.B = data.B[:length]
	}
	return data
}

func ByteSlicePut(data *ByteSlice) {
	data.B = data.B[:0]
	byteSlicePool.Put(data)
}
