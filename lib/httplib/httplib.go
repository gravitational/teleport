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

// Package httplib implements common utility functions for writing
// classic HTTP handlers
package httplib

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gravitational/teleport"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// HandlerFunc specifies HTTP handler function that returns error
type HandlerFunc func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error)

// MakeHandler returns a new httprouter.Handle func from a handler func
func MakeHandler(fn HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		out, err := fn(w, r, p)
		if err != nil {
			ReplyError(w, err)
			return
		}
		roundtrip.ReplyJSON(w, http.StatusOK, out)
	}
}

// ReadJSON reads HTTP json request and unmarshals it
// into passed interface{} obj
func ReadJSON(r *http.Request, val interface{}) error {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := json.Unmarshal(data, &val); err != nil {
		return trace.Wrap(teleport.BadParameter("request", err.Error()))
	}
	return nil
}

// ReplyError sets up http error response and writes it to writer w
func ReplyError(w http.ResponseWriter, err error) {
	if teleport.IsNotFound(err) {
		roundtrip.ReplyJSON(
			w, http.StatusNotFound, err)
	} else if teleport.IsBadParameter(err) {
		roundtrip.ReplyJSON(
			w, http.StatusBadRequest, err)
	} else if teleport.IsAccessDenied(err) {
		roundtrip.ReplyJSON(
			w, http.StatusForbidden, err)
	} else if teleport.IsAlreadyExists(err) {
		roundtrip.ReplyJSON(
			w, http.StatusConflict, err)
	} else {
		roundtrip.ReplyJSON(
			w, http.StatusInternalServerError, err)
	}
}

// ConvertResponse converts http error to internal error type
// based on HTTP response code and HTTP body contents
func ConvertResponse(re *roundtrip.Response, err error) (*roundtrip.Response, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch re.Code() {
	case http.StatusNotFound:
		e := teleport.NotFoundError{}
		unmarshalError(&e, re)
		return nil, trace.Wrap(&e)
	case http.StatusBadRequest:
		e := teleport.BadParameterError{}
		unmarshalError(&e, re)
		return nil, trace.Wrap(&e)
	case http.StatusForbidden:
		e := teleport.AccessDeniedError{}
		unmarshalError(&e, re)
		return nil, trace.Wrap(&e)
	case http.StatusConflict:
		e := teleport.AlreadyExistsError{}
		unmarshalError(&e, re)
		return nil, trace.Wrap(&e)
	}
	if re.Code() < 200 || re.Code() > 299 {
		return nil, trace.Wrap(
			teleport.BadParameter("errorcode",
				fmt.Sprintf("unrecognized http error: %v %v %v", err, re.Code(), string(re.Bytes()))))
	}
	return re, nil
}

func unmarshalError(err interface{}, re *roundtrip.Response) {
	err2 := json.Unmarshal(re.Bytes(), err)
	if err2 != nil {
		log.Infof("error unmarshaling response: '%v', err: %v", string(re.Bytes()), err2)
	}
}

// InsecureSetDevmodeHeaders allows cross-origin requests, used in dev mode only
func InsecureSetDevmodeHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Origin, Content-type, Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "1728000")
}
