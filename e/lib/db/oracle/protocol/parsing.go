package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
)

func readDataLengthContent(readInt func(reader *bytes.Reader) (int64, error), r *bytes.Reader) ([]byte, error) {
	dataLength, err := readInt(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if dataLength <= 0 {
		return nil, nil
	}

	out, err := readByteArray(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

func readKeyValueTag(readInt func(reader *bytes.Reader) (int64, error), r *bytes.Reader) (string, string, int64, error) {
	keyBuff, err := readDataLengthContent(readInt, r)
	if err != nil {
		return "", "", 0, trace.Wrap(err)
	}
	valBuff, err := readDataLengthContent(readInt, r)
	if err != nil {
		return "", "", 0, trace.Wrap(err)
	}

	tag, err := readInt(r)
	if err != nil {
		return "", "", 0, trace.Wrap(err)
	}

	return string(keyBuff), string(valBuff), tag, nil
}

func readByteArray(r *bytes.Reader) ([]byte, error) {
	// see https://github.com/oracle/python-oracledb/blob/deed9a338b8ea2721581846f025e4eed8dfc6d0a/src/oracledb/base_impl.pxd#L139-L141
	const longLengthIndicator = 254
	const nullLengthIndicator = 255

	length, err := r.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if length == 0 {
		return nil, nil
	}

	if length == nullLengthIndicator {
		return nil, nil
	}

	if length == longLengthIndicator {
		return nil, trace.NotImplemented("readByteArray: chunked decoding not implemented")
	}

	buff, err := readNBytes(r, int(length))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return buff, nil
}

func readString(r *bytes.Reader) (string, error) {
	buff, err := readByteArray(r)
	return string(buff), trace.Wrap(err)
}

func readVarInt64(r *bytes.Reader) (int64, error) {
	length, err := r.ReadByte()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	if length > 8 {
		return 0, trace.BadParameter("ReadVarInt64: invalid length value: %d, shouldn't be more than 8.", length)
	}

	temp := make([]byte, 8)
	offset := 8 - int(length)
	for i := range int(length) {
		var next byte
		next, err = r.ReadByte()
		if err != nil {
			return 0, trace.Wrap(err)
		}
		temp[offset+i] = next
	}
	return int64(binary.BigEndian.Uint64(temp)), nil
}

func readLittleEndianUint32(reader *bytes.Reader) (int64, error) {
	var dataLength uint32
	err := binary.Read(reader, binary.LittleEndian, &dataLength)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return int64(dataLength), nil
}

func readNBytes(r *bytes.Reader, length int) ([]byte, error) {
	buf := make([]byte, length)
	read, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if read != length {
		return nil, trace.BadParameter("Read %d bytes, expected %d", read, length)
	}
	return buf, nil
}
