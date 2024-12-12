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

package migration

import (
	"context"
	"sync"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
)

// NewCredentialsAdapter adapts an AWS SDK v2 credentials provider to v1
// credentials.
func NewCredentialsAdapter(providerV2 awsv2.CredentialsProvider) *credentials.Credentials {
	return credentials.NewCredentials(NewProviderAdapter(providerV2))
}

// NewProviderAdapter returns a [ProviderAdapter] that can be used as an AWS SDK
// v1 credentials provider.
func NewProviderAdapter(providerV2 awsv2.CredentialsProvider) *ProviderAdapter {
	return &ProviderAdapter{
		providerV2: providerV2,
	}
}

var _ credentials.ProviderWithContext = (*ProviderAdapter)(nil)

// ProviderAdapter adapts an [awsv2.CredentialsProvider] to an AWS SDK v1
// credentials provider.
type ProviderAdapter struct {
	providerV2 awsv2.CredentialsProvider

	m sync.RWMutex
	// creds are retrieved and saved to satisfy IsExpired.
	creds awsv2.Credentials
}

func (a *ProviderAdapter) IsExpired() bool {
	a.m.RLock()
	defer a.m.RUnlock()

	var emptyCreds awsv2.Credentials
	return a.creds == emptyCreds || a.creds.Expired()
}

func (a *ProviderAdapter) Retrieve() (credentials.Value, error) {
	return a.RetrieveWithContext(context.Background())
}

func (a *ProviderAdapter) RetrieveWithContext(ctx context.Context) (credentials.Value, error) {
	creds, err := a.retrieveLocked(ctx)
	if err != nil {
		return credentials.Value{}, trace.Wrap(err)
	}

	return credentials.Value{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		ProviderName:    creds.Source,
	}, nil
}

func (a *ProviderAdapter) retrieveLocked(ctx context.Context) (awsv2.Credentials, error) {
	a.m.Lock()
	defer a.m.Unlock()

	var emptyCreds awsv2.Credentials
	if a.creds != emptyCreds && !a.creds.Expired() {
		return a.creds, nil
	}

	creds, err := a.providerV2.Retrieve(ctx)
	if err != nil {
		return emptyCreds, trace.Wrap(err)
	}

	a.creds = creds
	return creds, nil
}
