/*
Copyright 2015 Gravitational, Inc.

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

package services

// Injector injects new data into server, or otherwise updates it
type Injector interface {
	Inject(s Server) error
}

// NopInjector does nothing
type NopInjector struct {
}

func (*NopInjector) Inject(s Server) error {
	return nil
}
