package mysql

import (
	"strconv"

	"github.com/go-mysql-org/go-mysql/utils"
	"github.com/pingcap/errors"
)

type RowData []byte

func (p RowData) Parse(f []*Field, binary bool, dst []FieldValue) ([]FieldValue, error) {
	if binary {
		return p.ParseBinary(f, dst)
	} else {
		return p.ParseText(f, dst)
	}
}

func (p RowData) ParseText(f []*Field, dst []FieldValue) ([]FieldValue, error) {
	for len(dst) < len(f) {
		dst = append(dst, FieldValue{})
	}
	data := dst[:len(f)]

	var err error
	var v []byte
	var isNull bool
	var pos int = 0
	var n int = 0

	for i := range f {
		v, isNull, n, err = LengthEncodedString(p[pos:])
		if err != nil {
			return nil, errors.Trace(err)
		}

		pos += n

		if isNull {
			data[i].Type = FieldValueTypeNull
		} else {
			isUnsigned := f[i].Flag&UNSIGNED_FLAG != 0

			switch f[i].Type {
			case MYSQL_TYPE_TINY, MYSQL_TYPE_SHORT, MYSQL_TYPE_INT24,
				MYSQL_TYPE_LONGLONG, MYSQL_TYPE_LONG, MYSQL_TYPE_YEAR:
				if isUnsigned {
					var val uint64
					data[i].Type = FieldValueTypeUnsigned
					val, err = strconv.ParseUint(utils.ByteSliceToString(v), 10, 64)
					data[i].value = val
				} else {
					var val int64
					data[i].Type = FieldValueTypeSigned
					val, err = strconv.ParseInt(utils.ByteSliceToString(v), 10, 64)
					data[i].value = utils.Int64ToUint64(val)
				}
			case MYSQL_TYPE_FLOAT, MYSQL_TYPE_DOUBLE:
				var val float64
				data[i].Type = FieldValueTypeFloat
				val, err = strconv.ParseFloat(utils.ByteSliceToString(v), 64)
				data[i].value = utils.Float64ToUint64(val)
			default:
				data[i].Type = FieldValueTypeString
				data[i].str = append(data[i].str[:0], v...)
			}

			if err != nil {
				return nil, errors.Trace(err)
			}
		}
	}

	return data, nil
}

// ParseBinary parses the binary format of data
// see https://dev.mysql.com/doc/internals/en/binary-protocol-value.html
func (p RowData) ParseBinary(f []*Field, dst []FieldValue) ([]FieldValue, error) {
	for len(dst) < len(f) {
		dst = append(dst, FieldValue{})
	}
	data := dst[:len(f)]

	if p[0] != OK_HEADER {
		return nil, ErrMalformPacket
	}

	pos := 1 + ((len(f) + 7 + 2) >> 3)

	nullBitmap := p[1:pos]

	var isNull bool
	var n int
	var err error
	var v []byte
	for i := range data {
		if nullBitmap[(i+2)/8]&(1<<(uint(i+2)%8)) > 0 {
			data[i].Type = FieldValueTypeNull
			continue
		}

		isUnsigned := f[i].Flag&UNSIGNED_FLAG != 0

		switch f[i].Type {
		case MYSQL_TYPE_NULL:
			data[i].Type = FieldValueTypeNull
			continue

		case MYSQL_TYPE_TINY:
			if isUnsigned {
				v := ParseBinaryUint8(p[pos : pos+1])
				data[i].Type = FieldValueTypeUnsigned
				data[i].value = uint64(v)
			} else {
				v := ParseBinaryInt8(p[pos : pos+1])
				data[i].Type = FieldValueTypeSigned
				data[i].value = utils.Int64ToUint64(int64(v))
			}
			pos++
			continue

		case MYSQL_TYPE_SHORT, MYSQL_TYPE_YEAR:
			if isUnsigned {
				v := ParseBinaryUint16(p[pos : pos+2])
				data[i].Type = FieldValueTypeUnsigned
				data[i].value = uint64(v)
			} else {
				v := ParseBinaryInt16(p[pos : pos+2])
				data[i].Type = FieldValueTypeSigned
				data[i].value = utils.Int64ToUint64(int64(v))
			}
			pos += 2
			continue

		case MYSQL_TYPE_INT24, MYSQL_TYPE_LONG:
			if isUnsigned {
				v := ParseBinaryUint32(p[pos : pos+4])
				data[i].Type = FieldValueTypeUnsigned
				data[i].value = uint64(v)
			} else {
				v := ParseBinaryInt32(p[pos : pos+4])
				data[i].Type = FieldValueTypeSigned
				data[i].value = utils.Int64ToUint64(int64(v))
			}
			pos += 4
			continue

		case MYSQL_TYPE_LONGLONG:
			if isUnsigned {
				v := ParseBinaryUint64(p[pos : pos+8])
				data[i].Type = FieldValueTypeUnsigned
				data[i].value = v
			} else {
				v := ParseBinaryInt64(p[pos : pos+8])
				data[i].Type = FieldValueTypeSigned
				data[i].value = utils.Int64ToUint64(v)
			}
			pos += 8
			continue

		case MYSQL_TYPE_FLOAT:
			v := ParseBinaryFloat32(p[pos : pos+4])
			data[i].Type = FieldValueTypeFloat
			data[i].value = utils.Float64ToUint64(float64(v))
			pos += 4
			continue

		case MYSQL_TYPE_DOUBLE:
			v := ParseBinaryFloat64(p[pos : pos+8])
			data[i].Type = FieldValueTypeFloat
			data[i].value = utils.Float64ToUint64(v)
			pos += 8
			continue

		case MYSQL_TYPE_DECIMAL, MYSQL_TYPE_NEWDECIMAL, MYSQL_TYPE_VARCHAR,
			MYSQL_TYPE_BIT, MYSQL_TYPE_ENUM, MYSQL_TYPE_SET, MYSQL_TYPE_TINY_BLOB,
			MYSQL_TYPE_MEDIUM_BLOB, MYSQL_TYPE_LONG_BLOB, MYSQL_TYPE_BLOB,
			MYSQL_TYPE_VAR_STRING, MYSQL_TYPE_STRING, MYSQL_TYPE_GEOMETRY:
			v, isNull, n, err = LengthEncodedString(p[pos:])
			pos += n
			if err != nil {
				return nil, errors.Trace(err)
			}

			if !isNull {
				data[i].Type = FieldValueTypeString
				data[i].str = append(data[i].str[:0], v...)
				continue
			} else {
				data[i].Type = FieldValueTypeNull
				continue
			}

		case MYSQL_TYPE_DATE, MYSQL_TYPE_NEWDATE:
			var num uint64
			num, isNull, n = LengthEncodedInt(p[pos:])

			pos += n

			if isNull {
				data[i].Type = FieldValueTypeNull
				continue
			}

			data[i].Type = FieldValueTypeString
			data[i].str, err = FormatBinaryDate(int(num), p[pos:])
			pos += int(num)

			if err != nil {
				return nil, errors.Trace(err)
			}

		case MYSQL_TYPE_TIMESTAMP, MYSQL_TYPE_DATETIME:
			var num uint64
			num, isNull, n = LengthEncodedInt(p[pos:])

			pos += n

			if isNull {
				data[i].Type = FieldValueTypeNull
				continue
			}

			data[i].Type = FieldValueTypeString
			data[i].str, err = FormatBinaryDateTime(int(num), p[pos:])
			pos += int(num)

			if err != nil {
				return nil, errors.Trace(err)
			}

		case MYSQL_TYPE_TIME:
			var num uint64
			num, isNull, n = LengthEncodedInt(p[pos:])

			pos += n

			if isNull {
				data[i].Type = FieldValueTypeNull
				continue
			}

			data[i].Type = FieldValueTypeString
			data[i].str, err = FormatBinaryTime(int(num), p[pos:])
			pos += int(num)

			if err != nil {
				return nil, errors.Trace(err)
			}

		default:
			return nil, errors.Errorf("Stmt Unknown FieldType %d %s", f[i].Type, f[i].Name)
		}
	}

	return data, nil
}
