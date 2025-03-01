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

package aws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/mocks"
)

// TestGetIdentity verifies parsing of AWS identity received from STS API.
func TestGetIdentity(t *testing.T) {
	tests := []struct {
		description  string
		inARN        string
		outIdentity  Identity
		outName      string
		outAccountID string
		outPartition string
		outType      string
	}{
		{
			description:  "role identity",
			inARN:        "arn:aws:iam::123456789012:role/custom/path/EC2ReadOnly",
			outIdentity:  Role{},
			outName:      "EC2ReadOnly",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "role",
		},
		{
			description:  "assumed role identity",
			inARN:        "arn:aws:sts::123456789012:assumed-role/DatabaseAccess/i-1234567890",
			outIdentity:  Role{},
			outName:      "DatabaseAccess",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "assumed-role",
		},
		{
			description:  "user identity",
			inARN:        "arn:aws-us-gov:iam::123456789012:user/custom/path/alice",
			outIdentity:  User{},
			outName:      "alice",
			outAccountID: "123456789012",
			outPartition: "aws-us-gov",
			outType:      "user",
		},
		{
			description:  "unsupported identity",
			inARN:        "arn:aws:iam::123456789012:group/readers",
			outIdentity:  Unknown{},
			outName:      "readers",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "group",
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			identity, err := GetIdentityWithClient(context.Background(), &mocks.STSClient{ARN: test.inARN})
			require.NoError(t, err)
			require.IsType(t, test.outIdentity, identity)
			require.Equal(t, test.outName, identity.GetName())
			require.Equal(t, test.outAccountID, identity.GetAccountID())
			require.Equal(t, test.outPartition, identity.GetPartition())
			require.Equal(t, test.outType, identity.GetType())
		})
	}
}
