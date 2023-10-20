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

// Package img contains the required interfaces and logic to validate that an
// image has been issued by Teleport and can be trusted.
//
// There are multiple image signature mechanisms competing, the Docker Content
// Trust (DCT) stores signatures in the manifest index itself. This approach is
// more  constraining because the signed object and the original object end up
// having different Digests. Signatures cannot be added or modified a
// posteriori.
//
// Cosign is another emerging signature standard, its signature specification is
// described here: https://github.com/sigstore/cosign/blob/main/specs/SIGNATURE_SPEC.md
// Cosign signatures are stored as images in the artifact registry, alongside
// the signed image. As signatures are stored separately, an object can be
// signed multiple times and this does not affect its digest.
// At the time of writing, cosign supports both static key signing and
// "keyless" (short-lived certificates) signing through the Fulcio CA.
// Keyless singing is still experimental and we don't use it yet, support might
// be added in the future.
//
// The Cosign validator delegates all validation logic to the upstream cosign
// implementation. This is a good thing as we benefit from all the bug fixes
// and security audits they make. However, we still to test against a
// wide range of valid and invalid signatures to make sure we invoke cosign
// properly and validate all fields and claims. As there are no tools to craft
// broken and invalid signatures, we needed to write out own fixture generator.
// Fixture generation is done by
// `teleport/integrations/kube-agent-updater/hack/cosign-fixtures.go`. The
// executable generates a go file declaring all blobs and manifests to be
// loaded in the test registry.
package img
