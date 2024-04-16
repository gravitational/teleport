/*
Copyright 2024 Gravitational, Inc.

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

package prometheus

import (
	_ "net/http"
	_ "unsafe"
)

// Link local error variables to unexported errors from http package by using
// go:linkname directive.

//go:linkname errTimeout net/http.errTimeout
var errTimeout error

//go:linkname errRequestCanceled net/http.errRequestCanceled
var errRequestCanceled error

//go:linkname errRequestCanceledConn net/http.errRequestCanceledConn
var errRequestCanceledConn error
