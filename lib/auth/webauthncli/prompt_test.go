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

package webauthncli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/stretchr/testify/assert"

	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

func TestDefaultPrompt_PromptCredential(t *testing.T) {
	oldStdin := prompt.Stdin()
	t.Cleanup(func() { prompt.SetStdin(oldStdin) })

	llamaCred := &wancli.CredentialInfo{
		User: wancli.UserInfo{
			Name: "llama",
		},
	}
	alpacaCred := &wancli.CredentialInfo{
		User: wancli.UserInfo{
			Name: "alpaca",
		},
	}
	camelCred := &wancli.CredentialInfo{
		User: wancli.UserInfo{
			Name: "camel",
		},
	}

	ctx := context.Background()

	tests := []struct {
		name       string
		fakeReader *prompt.FakeReader
		ctx        context.Context
		creds      []*wancli.CredentialInfo
		wantCred   *wancli.CredentialInfo
		wantErr    string
		// Optional, verifies output text.
		wantOut string
	}{
		{
			name:       "credential by number (1)",
			fakeReader: prompt.NewFakeReader().AddString("1"), // sorted by name
			creds:      []*wancli.CredentialInfo{llamaCred, alpacaCred, camelCred},
			wantCred:   alpacaCred,
		},
		{
			name:       "credential by number (2)",
			fakeReader: prompt.NewFakeReader().AddString("3"), // sorted by name
			creds:      []*wancli.CredentialInfo{llamaCred, alpacaCred, camelCred},
			wantCred:   llamaCred,
		},
		{
			name:       "credential by name",
			fakeReader: prompt.NewFakeReader().AddString("alpaca"),
			creds:      []*wancli.CredentialInfo{llamaCred, alpacaCred, camelCred},
			wantCred:   alpacaCred,
		},
		{
			name: "loops until correct",
			fakeReader: prompt.NewFakeReader().
				AddString("bad").
				AddString("5").
				AddString("llama"),
			creds:    []*wancli.CredentialInfo{llamaCred, alpacaCred, camelCred},
			wantCred: llamaCred,
		},
		{
			name:       "NOK empty credentials errors",
			fakeReader: prompt.NewFakeReader(),
			creds:      []*wancli.CredentialInfo{},
			wantErr:    "empty credentials",
		},
		{
			name:       "output text",
			fakeReader: prompt.NewFakeReader().AddString("llama"),
			creds:      []*wancli.CredentialInfo{llamaCred, alpacaCred, camelCred},
			wantCred:   llamaCred,
			wantOut: `[1] alpaca
[2] camel
[3] llama
` + wancli.NewDefaultPrompt(ctx, nil).PromptCredentialMessage,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prompt.SetStdin(test.fakeReader)

			out := &strings.Builder{}
			p := wancli.NewDefaultPrompt(ctx, out)
			got, err := p.PromptCredential(test.creds)
			switch {
			case err == nil && test.wantErr == "": // OK
			case err == nil && test.wantErr != "":
				fallthrough
			case !strings.Contains(err.Error(), test.wantErr):
				t.Fatalf("PromptCredential returned err = %v, want %q", err, test.wantErr)
			}
			assert.Equal(t, test.wantCred, got, "PromptCredential mismatch")
			if test.wantOut != "" {
				// Contains so we don't trip on punctuation from prompt.Input.
				assert.Contains(t, out.String(), test.wantOut, "output mismatch")
			}
		})
	}
}
