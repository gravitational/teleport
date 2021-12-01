package mysql

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/pingcap/errors"
	"github.com/siddontang/go/hack"
)

type Resultset struct {
	Fields     []*Field
	FieldNames map[string]int
	Values     [][]FieldValue

	RawPkg []byte

	RowDatas []RowData
}

var (
	resultsetPool = sync.Pool{
		New: func() interface{} {
			return &Resultset{}
		},
	}
)

func NewResultset(fieldsCount int) *Resultset {
	r := resultsetPool.Get().(*Resultset)
	r.Reset(fieldsCount)
	return r
}

func (r *Resultset) returnToPool() {
	resultsetPool.Put(r)
}

func (r *Resultset) Reset(fieldsCount int) {
	r.RawPkg = r.RawPkg[:0]

	r.Fields = r.Fields[:0]
	r.Values = r.Values[:0]
	r.RowDatas = r.RowDatas[:0]

	if r.FieldNames != nil {
		for k := range r.FieldNames {
			delete(r.FieldNames, k)
		}
	} else {
		r.FieldNames = make(map[string]int)
	}

	if fieldsCount == 0 {
		return
	}

	if cap(r.Fields) < fieldsCount {
		r.Fields = make([]*Field, fieldsCount)
	} else {
		r.Fields = r.Fields[:fieldsCount]
	}
}

func (r *Resultset) RowNumber() int {
	return len(r.Values)
}

func (r *Resultset) ColumnNumber() int {
	return len(r.Fields)
}

func (r *Resultset) GetValue(row, column int) (interface{}, error) {
	if row >= len(r.Values) || row < 0 {
		return nil, errors.Errorf("invalid row index %d", row)
	}

	if column >= len(r.Fields) || column < 0 {
		return nil, errors.Errorf("invalid column index %d", column)
	}

	return r.Values[row][column].Value(), nil
}

func (r *Resultset) NameIndex(name string) (int, error) {
	if column, ok := r.FieldNames[name]; ok {
		return column, nil
	} else {
		return 0, errors.Errorf("invalid field name %s", name)
	}
}

func (r *Resultset) GetValueByName(row int, name string) (interface{}, error) {
	if column, err := r.NameIndex(name); err != nil {
		return nil, errors.Trace(err)
	} else {
		return r.GetValue(row, column)
	}
}

func (r *Resultset) IsNull(row, column int) (bool, error) {
	d, err := r.GetValue(row, column)
	if err != nil {
		return false, err
	}

	return d == nil, nil
}

func (r *Resultset) IsNullByName(row int, name string) (bool, error) {
	if column, err := r.NameIndex(name); err != nil {
		return false, err
	} else {
		return r.IsNull(row, column)
	}
}

func (r *Resultset) GetUint(row, column int) (uint64, error) {
	d, err := r.GetValue(row, column)
	if err != nil {
		return 0, err
	}

	switch v := d.(type) {
	case int:
		return uint64(v), nil
	case int8:
		return uint64(v), nil
	case int16:
		return uint64(v), nil
	case int32:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float32:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	case string:
		return strconv.ParseUint(v, 10, 64)
	case []byte:
		return strconv.ParseUint(string(v), 10, 64)
	case nil:
		return 0, nil
	default:
		return 0, errors.Errorf("data type is %T", v)
	}
}

func (r *Resultset) GetUintByName(row int, name string) (uint64, error) {
	if column, err := r.NameIndex(name); err != nil {
		return 0, err
	} else {
		return r.GetUint(row, column)
	}
}

func (r *Resultset) GetInt(row, column int) (int64, error) {
	v, err := r.GetUint(row, column)
	if err != nil {
		return 0, err
	}

	return int64(v), nil
}

func (r *Resultset) GetIntByName(row int, name string) (int64, error) {
	v, err := r.GetUintByName(row, name)
	if err != nil {
		return 0, err
	}

	return int64(v), nil
}

func (r *Resultset) GetFloat(row, column int) (float64, error) {
	d, err := r.GetValue(row, column)
	if err != nil {
		return 0, err
	}

	switch v := d.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	case []byte:
		return strconv.ParseFloat(string(v), 64)
	case nil:
		return 0, nil
	default:
		return 0, errors.Errorf("data type is %T", v)
	}
}

func (r *Resultset) GetFloatByName(row int, name string) (float64, error) {
	if column, err := r.NameIndex(name); err != nil {
		return 0, err
	} else {
		return r.GetFloat(row, column)
	}
}

func (r *Resultset) GetString(row, column int) (string, error) {
	d, err := r.GetValue(row, column)
	if err != nil {
		return "", err
	}

	switch v := d.(type) {
	case string:
		return v, nil
	case []byte:
		return hack.String(v), nil
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case nil:
		return "", nil
	default:
		return "", errors.Errorf("data type is %T", v)
	}
}

func (r *Resultset) GetStringByName(row int, name string) (string, error) {
	if column, err := r.NameIndex(name); err != nil {
		return "", err
	} else {
		return r.GetString(row, column)
	}
}
