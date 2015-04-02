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
