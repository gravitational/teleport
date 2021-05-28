/*
Copyright 2017-2021 Gravitational, Inc.

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

// package modules allows external packages override certain behavioral
// aspects of teleport
package modules

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"runtime"
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// Features provides supported and unsupported features
type Features struct {
	// Kubernetes enables Kubernetes Access product
	Kubernetes bool
	// App enables Application Access product
	App bool
	// DB enables database access product
	DB bool
	// OIDC enables OIDC connectors
	OIDC bool
	// SAML enables SAML connectors
	SAML bool
	// AccessControls enables FIPS access controls
	AccessControls bool
	// AdvancedAccessWorkflows enables advanced access workflows
	AdvancedAccessWorkflows bool
	// Cloud enables some cloud-related features
	Cloud bool
}

// ToProto converts Features into proto.Features
func (f Features) ToProto() *proto.Features {
	return &proto.Features{
		Kubernetes:              f.Kubernetes,
		App:                     f.App,
		DB:                      f.DB,
		OIDC:                    f.OIDC,
		SAML:                    f.SAML,
		AccessControls:          f.AccessControls,
		AdvancedAccessWorkflows: f.AdvancedAccessWorkflows,
		Cloud:                   f.Cloud,
	}
}

// Modules defines interface that external libraries can implement customizing
// default teleport behavior
type Modules interface {
	// PrintVersion prints teleport version
	PrintVersion()
	// IsBoringBinary checks if the binary was compiled with BoringCrypto.
	IsBoringBinary() bool
	// Features returns supported features
	Features() Features
	// BuildType returns build type (OSS or Enterprise)
	BuildType() string
	// ValidateResource performs additional resource checks.
	ValidateResource(types.Resource) error
}

const (
	// BuildOSS specifies open source build type
	BuildOSS = "oss"
	// BuildEnterprise specifies enterprise build type
	BuildEnterprise = "ent"
)

// SetModules sets the modules interface
func SetModules(m Modules) {
	mutex.Lock()
	defer mutex.Unlock()
	modules = m
}

// GetModules returns the modules interface
func GetModules() Modules {
	mutex.Lock()
	defer mutex.Unlock()
	return modules
}

type defaultModules struct{}

// BuildType returns build type (OSS or Enterprise)
func (p *defaultModules) BuildType() string {
	return BuildOSS
}

// PrintVersion prints the Teleport version.
func (p *defaultModules) PrintVersion() {
	fmt.Printf("Teleport v%s git:%s %s\n", teleport.Version, teleport.Gitref, runtime.Version())
}

// Features returns supported features
func (p *defaultModules) Features() Features {
	return Features{
		Kubernetes: true,
		DB:         true,
		App:        true,
	}
}

func (p *defaultModules) IsBoringBinary() bool {
	// Check the package name for one of the boring primitives, if the package
	// path is from BoringCrypto, we know this binary was compiled against the
	// dev.boringcrypto branch of Go.
	hash := sha256.New()
	return reflect.TypeOf(hash).Elem().PkgPath() == "crypto/internal/boring"
}

// ValidateResource performs additional resource checks.
func (p *defaultModules) ValidateResource(types.Resource) error {
	return nil
}

var (
	mutex   sync.Mutex
	modules Modules = &defaultModules{}
)
