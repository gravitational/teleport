package backend

import (
	"encoding/json"
	"time"
)

type JSONCodec struct {
	Backend
}

func (c *JSONCodec) UpsertJSONVal(path []string, key string, val interface{}, ttl time.Duration) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.UpsertVal(path, key, bytes, ttl)
}

func (c *JSONCodec) GetJSONVal(path []string, key string, val interface{}) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	bytes, err = c.GetVal(path, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, val)
}
