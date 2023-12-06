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

package utils

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// GetRawEC2IdentityDocument fetches the PKCS7 RSA2048 InstanceIdentityDocument
// from the IMDS for this EC2 instance.
func GetRawEC2IdentityDocument(ctx context.Context) ([]byte, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	imdsClient := imds.NewFromConfig(cfg)
	output, err := imdsClient.GetDynamicData(ctx, &imds.GetDynamicDataInput{
		Path: "instance-identity/rsa2048",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iidBytes, err := ReadAtMost(output.Content, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := output.Content.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	return iidBytes, nil
}

func GetEC2InstanceIdentityDocument(ctx context.Context) (*imds.InstanceIdentityDocument, error) {
	// fetch the raw IID
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	imdsClient := imds.NewFromConfig(cfg)
	output, err := imdsClient.GetInstanceIdentityDocument(ctx, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &output.InstanceIdentityDocument, nil
}

// GetEC2NodeID returns the node ID to use for this EC2 instance when using
// Simplified Node Joining.
func GetEC2NodeID(ctx context.Context) (string, error) {
	// fetch the raw IID
	iid, err := GetEC2InstanceIdentityDocument(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return NodeIDFromIID(iid), nil
}

// NodeIDFromIID returns the node ID that must be used for nodes joining with
// the given Instance Identity Document.
func NodeIDFromIID(iid *imds.InstanceIdentityDocument) string {
	return iid.AccountID + "-" + iid.InstanceID
}
