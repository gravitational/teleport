/*
Copyright 2023 Gravitational, Inc.

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

package okta

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/sirupsen/logrus"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
)

// Config is the configuration for the Okta service.
type Config struct {
	// Log is the logger for the Okta config.
	Log *logrus.Entry

	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter

	// AccessPoint is the caching access point for the Okta service.
	AccessPoint auth.OktaAccessPoint

	// OktaAPIEndpoint is the API endpoint to use for interacting with Okta.
	OktaAPIEndpoint string

	// OktaAPIToken is the API token.
	OktaAPIToken string
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, teleport.ComponentOkta)
	}
	if c.Emitter == nil {
		return trace.BadParameter("emitter is missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}
	if c.OktaAPIEndpoint == "" {
		return trace.BadParameter("Okta API endpoint is missing")
	}
	if c.OktaAPIToken == "" {
		return trace.BadParameter("Okta API token is missing")
	}
	return nil
}

// Service is the Okta service.
type Service struct {
	log         logrus.FieldLogger
	accessPoint auth.OktaAccessPoint
	client      *okta.Client
	emitter     apievents.Emitter

	stopCh chan struct{}
}

// New will create a new Okta service.
func New(ctx context.Context, config Config) (*Service, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	_, client, err := okta.NewClient(ctx,
		okta.WithOrgUrl(config.OktaAPIEndpoint),
		okta.WithToken(config.OktaAPIToken),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:         config.Log,
		accessPoint: config.AccessPoint,
		client:      client,
		emitter:     config.Emitter,
		stopCh:      make(chan struct{}, 1),
	}, nil
}

// Start will start the Okta service.
// TODO(mdwn): Implement this stub.
func (s *Service) Start(_ context.Context) error {
	return nil
}

// Stop will stop the Okta service.
// TODO(mdwn): Implement this stub.
func (s *Service) Stop() {
	s.stopCh <- struct{}{}
}

// Wait will wait for the Okta service to complete.
// TODO(mdwn): Implement this stub.
func (s *Service) Wait(ctx context.Context) error {
	select {
	case <-s.stopCh:
	case <-ctx.Done():
	}
	return nil
}
