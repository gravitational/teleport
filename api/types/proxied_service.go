/*
Copyright 2022 Gravitational, Inc.

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

package types

// ProxiedService is a service that is connected to a proxy.
type ProxiedService interface {
	// GetProxyIDs returns a list of proxy ids this service is connected to.
	GetProxyIDs() []string
	// SetProxyIDs sets the proxy ids this service is connected to.
	SetProxyIDs([]string)
}
