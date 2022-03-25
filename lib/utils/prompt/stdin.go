/*
Copyright 2021 Gravitational, Inc.

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

package prompt

import (
	"os"
	"sync"
)

var (
	stdinOnce = &sync.Once{}
	stdin     *ContextReader
)

// Stdin returns a singleton ContextReader wrapped around os.Stdin.
//
// os.Stdin should not be used directly after the first call to this function
// to avoid losing data. Closing this ContextReader will prevent all future
// reads for all callers.
func Stdin() *ContextReader {
	stdinOnce.Do(func() {
		stdin = NewContextReader(os.Stdin)
	})
	return stdin
}
