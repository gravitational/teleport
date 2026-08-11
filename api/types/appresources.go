/*
Copyright 2026 Gravitational, Inc.

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

// IsAllowAllOnly reports whether the rule sets allow_all and no other
// field, known or unknown. Fields unknown to this version could
// restrict the rule, so a rule carrying any does not count as
// unrestricted.
func (a AppResource) IsAllowAllOnly() bool {
	return a.AllowAll && len(a.XXX_unrecognized) == 0
}

// AppResourcesAllowAll reports whether a role's app rules grant full
// unrestricted app access. It requires a single allow rule that sets
// allow_all and no deny-side rules.
//
// Deny-side rules and multiple allow rules cannot be written in this
// version, but a newer version may introduce them. Returning false on
// their presence keeps this version from treating such a role as
// unrestricted.
func AppResourcesAllowAll(allow, deny []AppResource) bool {
	return len(deny) == 0 && len(allow) == 1 && allow[0].IsAllowAllOnly()
}
