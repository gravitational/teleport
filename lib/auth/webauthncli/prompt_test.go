/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package webauthncli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/touchid"
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

type funcToPicker func([]*wancli.CredentialInfo) (*wancli.CredentialInfo, error)

func (f funcToPicker) PromptCredential(creds []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
	return f(creds)
}

func TestToTouchIDCredentialPicker(t *testing.T) {
	indexPicker := func(i int) func([]*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
		return func(creds []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
			return creds[i], nil
		}
	}
	errorPicker := func(err error) func([]*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
		return func(_ []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
			return nil, err
		}
	}
	bogusPicker := func(resp *wancli.CredentialInfo) func([]*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
		return func(_ []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
			return resp, nil
		}
	}

	creds := []*touchid.CredentialInfo{
		{
			CredentialID: "id1",
			User: touchid.UserInfo{
				UserHandle: []byte("llama"),
				Name:       "llama",
			},
		},
		{
			CredentialID: "id2",
			User: touchid.UserInfo{
				UserHandle: []byte("alpaca"),
				Name:       "alpaca",
			},
		},
		{
			CredentialID: "id3",
			User: touchid.UserInfo{
				UserHandle: []byte("camel"),
				Name:       "camel",
			},
		},
		{
			CredentialID: "id4",
			User: touchid.UserInfo{
				UserHandle: []byte("llama"),
				Name:       "llama",
			},
		},
	}

	tests := []struct {
		name     string
		picker   func([]*wancli.CredentialInfo) (*wancli.CredentialInfo, error)
		creds    []*touchid.CredentialInfo
		wantCred *touchid.CredentialInfo
		wantErr  string
	}{
		{
			name:     "picks first credential",
			picker:   indexPicker(0),
			creds:    creds,
			wantCred: creds[0],
		},
		{
			name:     "picks middle credential",
			picker:   indexPicker(1),
			creds:    creds,
			wantCred: creds[1],
		},
		{
			name:     "picks last credential",
			picker:   indexPicker(3),
			creds:    creds,
			wantCred: creds[3],
		},
		{
			name:    "picker errors",
			picker:  errorPicker(errors.New("something real bad happened")),
			creds:   creds,
			wantErr: "something real bad happened",
		},
		{
			name: "picker returns bogus credential",
			picker: bogusPicker(&wancli.CredentialInfo{
				// It doesn't matter that the fields match, the pointer is not present
				// in the original array.
				ID: []byte(creds[0].CredentialID),
				User: wancli.UserInfo{
					UserHandle: creds[0].User.UserHandle,
					Name:       creds[0].User.Name,
				},
			}),
			creds:   creds,
			wantErr: "returned invalid credential",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			picker := wancli.ToTouchIDCredentialPicker(funcToPicker(test.picker))
			got, err := picker.PromptCredential(test.creds)
			if test.wantErr == "" {
				assert.NoError(t, err, "PromptCredential error mismatch")
			} else {
				assert.ErrorContains(t, err, test.wantErr, "PromptCredential error mismatch")
			}
			assert.Equal(t, test.wantCred, got, "PromptCredential cred mismatch")
		})
	}

	t.Run("credentials converted correctly", func(t *testing.T) {
		picker := wancli.ToTouchIDCredentialPicker(funcToPicker(
			func(ci []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
				require.Len(t, ci, len(creds), "creds length mismatch")
				for i, c := range ci {
					other := creds[i]

					// We are bordering a change detection test here, so let's just check
					// the ID and make sure the fields we care about aren't empty.
					assert.Equal(t, []byte(other.CredentialID), c.ID, "creds[%v].CredentialID mismatch", i)
					assert.NotEmpty(t, c.User.UserHandle, "creds[%v].User.UserHandle empty", i)
					assert.NotEmpty(t, c.User.Name, "creds[%v].User.Name empty", i)
				}
				return ci[0], nil
			}))

		_, err := picker.PromptCredential(creds)
		require.NoError(t, err, "PromptCredential errored unexpectedly")
	})
}
