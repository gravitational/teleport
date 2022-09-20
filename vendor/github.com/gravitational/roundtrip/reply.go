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
package roundtrip

import (
	"encoding/json"
	"net/http"
)

// ReplyJSON encodes the passed objec as application/json and writes
// a reply with a given HTTP status code to `w`
//
//   ReplyJSON(w, 404, map[string]interface{}{"msg": "not found"})
//
func ReplyJSON(w http.ResponseWriter, code int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	out, err := json.Marshal(obj)
	if err != nil {
		out = []byte(`{"msg": "internal marshal error"}`)
	}
	w.Write(out)
}
