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
package utils

import (
	"crypto/rand"
	"fmt"
	"io"
)

// NewUUID() returns a generated UUID string which looks like
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
func NewUUID() string {
	bytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		panic(err.Error())
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:])
}
