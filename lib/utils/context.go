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

package utils

import "context"

// AddFlagToContext is a generic helper to enable a struct flag type to the
// provided context.
func AddFlagToContext[FlagType any](parent context.Context) context.Context {
	return context.WithValue(parent, (*FlagType)(nil), (*FlagType)(nil))
}

// GetFlagFromContext is a generic helper that returns true if provided struct
// flag type is set.
func GetFlagFromContext[FlagType any](ctx context.Context) bool {
	_, ok := ctx.Value((*FlagType)(nil)).(*FlagType)
	return ok
}
