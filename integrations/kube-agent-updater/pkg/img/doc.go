/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
// Keyless Signing is still experimental and we don't use it yet, support might
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
