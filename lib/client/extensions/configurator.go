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

// Package extensions provides the Teleport client with additional functionality
// such as the means for configuring access to extra services that may be supported
// by a teleport cluster.
//
// For instance, in certain cases the cluster's proxy server may implement
// Docker registry or Helm chart repository support, in which case Docker
// and Helm clients will be configured with proper access credentials upon
// successful tsh login.
package extensions

import (
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "client:ext")

// Configurator defines an interface for configuring additional services
// provided by a Teleport server such as a Docker registry or Helm chart
// repository.
type Configurator interface {
	// Configure performs necessary service configuration.
	Configure(Config) error
	// Deconfigure removes configuration for the service.
	Deconfigure(Config) error
	// Stringer returns human-friendly description of the configurator.
	fmt.Stringer
}

// Config represents a service configuration parameters.
type Config struct {
	// ProxyAddress is the address of web proxy that provides the service.
	ProxyAddress string
	// CertificatePath is the full path to the client certificate file.
	CertificatePath string
	// KeyPath is the full path to the client private key file.
	KeyPath string
}

// NewDockerConfigurator returns a new instance of a Docker configurator.
func NewDockerConfigurator() *dockerConfigurator {
	return &dockerConfigurator{}
}

// NewHelmConfigurator returns a new instance of a Helm configurator.
func NewHelmConfigurator() *helmConfigurator {
	return &helmConfigurator{}
}
