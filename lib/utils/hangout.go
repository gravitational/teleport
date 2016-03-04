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

package utils

import (
	"encoding/base64"
	"encoding/json"

	"github.com/gravitational/trace"
)

type HangoutInfo struct {
	AuthPort  string `json:"auth_port"`
	NodePort  string `json:"node_port"`
	HangoutID string `json:"hangout_id"`
	OSUser    string `json:"os_user"`
}

func MarshalHangoutInfo(h *HangoutInfo) (string, error) {
	jsonString, err := json.Marshal(h)
	if err != nil {
		return "", trace.Wrap(err)
	}
	b64str := base64.StdEncoding.EncodeToString(jsonString)
	return string(b64str), nil
}

func UnmarshalHangoutInfo(id string) (*HangoutInfo, error) {
	jsonString, err := base64.StdEncoding.DecodeString(id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h HangoutInfo
	err = json.Unmarshal(jsonString, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &h, nil
}

const (
	HangoutAuthPortAlias = "auth"
	HangoutNodePortAlias = "node"
)
