/*
Copyright 2023 Gravitational, Inc.

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

package teleport

const (
	// ComponentSAMLIdP is a SAML identity provider component.
	//
	//nolint:revive // Because we want this to be IdP.
	ComponentSAMLIdP = "idp.saml"

	// ComponentOkta is an Okta service component.
	ComponentOkta = "okta"

	// ComponentOktaAssignmentReconciler is an Okta assignment reconciler component.
	ComponentOktaAssignmentReconciler = "okta.assignment-reconciler"

	// ComponentOktaAccessRequestReconciler is an access request reconciler component.
	ComponentOktaAccessRequestReconciler = "okta.access-request-reconciler"

	// ComponentOktaUserAssignmentCreator is a user assignment creator component.
	ComponentOktaUserAssignmentCreator = "okta.user-assignment-creator"
)
