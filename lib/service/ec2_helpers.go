/*
Copyright 2021 Gravitational, Inc.

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

package service

import (
	"context"
	"io"

	"github.com/gravitational/teleport/lib/auth"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/gravitational/trace"
)

// getEC2IdentityDocument fetches the PKCS7 RSA2048 InstanceIdentityDocument
// from the IMDS for this EC2 instance.
func getEC2IdentityDocument() ([]byte, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	imdsClient := imds.NewFromConfig(cfg)
	output, err := imdsClient.GetDynamicData(context.TODO(), &imds.GetDynamicDataInput{
		Path: "instance-identity/rsa2048",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iidBytes, err := io.ReadAll(output.Content)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := output.Content.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	return iidBytes, nil
}

// getEC2NodeID returns the node ID to use for this EC2 instance when using
// Simplified Node Joining.
func getEC2NodeID() (string, error) {
	// fetch the raw IID
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", trace.Wrap(err)
	}
	imdsClient := imds.NewFromConfig(cfg)
	output, err := imdsClient.GetInstanceIdentityDocument(context.TODO(), nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return auth.NodeIDFromIID(&output.InstanceIdentityDocument), nil
}
