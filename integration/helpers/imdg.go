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

package helpers

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// DisabledIMDSClient is an EC2 instance metadata client that is always disabled. This is faster
// than the default client when not testing instance metadata behavior.
type DisabledIMDSClient struct{}

func (d *DisabledIMDSClient) IsAvailable(ctx context.Context) bool {
	return false
}

func (d *DisabledIMDSClient) GetTags(ctx context.Context) (map[string]string, error) {
	return nil, nil
}

func (d *DisabledIMDSClient) GetHostname(ctx context.Context) (string, error) {
	return "", nil
}

func (d *DisabledIMDSClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeDisabled
}
