/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
