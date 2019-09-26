/*
Copyright 2019 Gravitational, Inc.

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

package configurator

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "configurator")

// Configurator defines an interface for configuring additional services
// provided by a Teleport server such as a Docker registry or Helm chart
// repository.
type Configurator interface {
	// Configure performs necessary service configuration.
	Configure(Config) error
	// IsConfigured returns true if the service is already configured.
	IsConfigured(Config) (bool, error)
}

// Config represents a service configuration parameters.
type Config struct {
	// ProxyAddress is the address of web proxy that provides the service.
	ProxyAddress string
	// ProfileDir is the full path to the client profile directory.
	ProfileDir string
	// CertificatePath is the full path to the client certificate file.
	CertificatePath string
	// KeyPath is the full path to the client private key file.
	KeyPath string
}

// NewDocker returns a new instance of Docker configurator.
func NewDocker(debug bool) (*dockerConfigurator, error) {
	return &dockerConfigurator{debug: debug}, nil
}

// NewHelm returns a new instance of Helm configurator.
func NewHelm() (*helmConfigurator, error) {
	return &helmConfigurator{}, nil
}
