// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webauthncli

import (
	"fmt"

	"github.com/gravitational/teleport/lib/auth/touchid"
	"github.com/gravitational/teleport/lib/auth/webauthncli/webauthnprompt"
	"github.com/gravitational/trace"
)

type credentialPicker interface {
	PromptCredential([]*webauthnprompt.CredentialInfo) (*webauthnprompt.CredentialInfo, error)
}

// ToTouchIDCredentialPicker adapts a wancli credential picker, such as
// LoginPrompt or DefaultPrompt, to a touchid.CredentialPicker
func ToTouchIDCredentialPicker(p credentialPicker) touchid.CredentialPicker {
	return tidPickerAdapter{impl: p}
}

type tidPickerAdapter struct {
	impl credentialPicker
}

func (p tidPickerAdapter) PromptCredential(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
	credMap := make(map[*webauthnprompt.CredentialInfo]*touchid.CredentialInfo)
	wcreds := make([]*webauthnprompt.CredentialInfo, len(creds))
	for i, c := range creds {
		cred := &webauthnprompt.CredentialInfo{
			ID: []byte(c.CredentialID),
			User: webauthnprompt.UserInfo{
				UserHandle: c.User.UserHandle,
				Name:       c.User.Name,
			},
		}
		credMap[cred] = c
		wcreds[i] = cred
	}

	wchoice, err := p.impl.PromptCredential(wcreds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	choice, ok := credMap[wchoice]
	if !ok {
		return nil, fmt.Errorf("prompt returned invalid credential: %#v", wchoice)
	}
	return choice, nil
}
