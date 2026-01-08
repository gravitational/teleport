// Copyright 2026 Gravitational, Inc.
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

package appserveropts

type EqualOptions struct {
	SkipClone    bool
	IgnoreHostID bool
}

func NewEqual(opts []EqualOpt) EqualOptions {
	opt := EqualOptions{}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

type EqualOpt func(*EqualOptions)

// WithSkipClone configures EqualAppServers to skip cloning and directly mutate the input app
// servers if necessary. Use this option only when you're certain the input app servers can be
// safely modified (e.g., they're already clones or will be discarded after comparison).
func WithSkipClone(skipClone bool) EqualOpt {
	return func(c *EqualOptions) {
		c.SkipClone = skipClone
	}
}

// WithIgnoreHostID when set to true forces to ignore differences in Spec.HostID while checking
// AppServer equality.
//
// NOTE: When specified the app servers passed to the EqualAppServers will be cloned. To avoid that
// [WithSkipClone] can be passed.
func WithIgnoreHostID(ignoreHostID bool) EqualOpt {
	return func(o *EqualOptions) {
		o.IgnoreHostID = ignoreHostID
	}
}
