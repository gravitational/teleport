// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package awsconfig

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Provider provides an [aws.Config].
type Provider interface {
	// GetConfig returns an [aws.Config] for the given region and options.
	GetConfig(ctx context.Context, region string, optFns ...OptionsFn) (aws.Config, error)
}

// ProviderFunc is a [Provider] adapter for functions.
type ProviderFunc func(ctx context.Context, region string, optFns ...OptionsFn) (aws.Config, error)

// GetConfig returns an [aws.Config] for the given region and options.
func (fn ProviderFunc) GetConfig(ctx context.Context, region string, optFns ...OptionsFn) (aws.Config, error) {
	return fn(ctx, region, optFns...)
}
