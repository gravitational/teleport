// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils_test

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/lib/utils"
)

func ExampleGetFlagFromContext() {
	type flag1 struct{}
	type flag2 struct{}

	ctx := utils.AddFlagToContext[flag1](context.Background())

	fmt.Println(utils.GetFlagFromContext[flag1](ctx))
	fmt.Println(utils.GetFlagFromContext[flag2](ctx))
	// Output:
	// true
	// false
}
