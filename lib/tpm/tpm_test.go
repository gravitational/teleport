/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tpm_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/tpm"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestPrintQuery(t *testing.T) {
	tests := []struct {
		name  string
		query *tpm.QueryRes
		debug bool
	}{
		{
			name: "ekpub",
			query: &tpm.QueryRes{
				EKPub:     []byte("ekpub"),
				EKPubHash: "aabbaabbcc",
			},
		},
		{
			name: "ekpub debug",
			query: &tpm.QueryRes{
				EKPub:     []byte("ekpub"),
				EKPubHash: "aabbaabbcc",
			},
			debug: true,
		},
		{
			name: "ekcert",
			query: &tpm.QueryRes{
				EKPub:     []byte("ekpub"),
				EKPubHash: "aabbaabbcc",
				EKCert: &tpm.QueryEKCert{
					Raw:          []byte("ekcert"),
					SerialNumber: "aa:bb:cc",
				},
			},
		},
		{
			name: "ekcert debug",
			query: &tpm.QueryRes{
				EKPub:     []byte("ekpub"),
				EKPubHash: "aabbaabbcc",
				EKCert: &tpm.QueryEKCert{
					Raw:          []byte("ekcert"),
					SerialNumber: "aa:bb:cc",
				},
			},
			debug: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			tpm.PrintQuery(tt.query, tt.debug, buf)
			if golden.ShouldSet() {
				golden.Set(t, buf.Bytes())
			}
			assert.Equal(t, string(golden.Get(t)), buf.String())
		})
	}
}
