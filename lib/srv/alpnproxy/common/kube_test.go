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

package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/idna"
)

// longKubeCluster is a 51-byte kube cluster name which, when hex-encoded,
// produces a 102-char DNS label and violates RFC 1035's 63-byte label limit.
// See issue #61439.
const longKubeCluster = "long-kube-cluster-name-exceeding-thirty-one-bytes-x"

func TestKubeLocalProxySNI_LabelLength(t *testing.T) {
	const teleportCluster = "example.teleport.sh"

	cases := []struct {
		name        string
		kubeCluster string
	}{
		{name: "short name", kubeCluster: "kube1"},
		{name: "long name", kubeCluster: longKubeCluster},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sni := KubeLocalProxySNI(teleportCluster, tc.kubeCluster)
			for label := range strings.SplitSeq(sni, ".") {
				require.LessOrEqualf(t, len(label), 63,
					"DNS label %q (%d bytes) exceeds RFC 1035 63-byte limit in SNI %q",
					label, len(label), sni)
			}
		})
	}
}

func TestKubeLocalProxySNI_ValidIDN(t *testing.T) {
	const teleportCluster = "example.teleport.sh"

	// Strict profile that mirrors Python's `idna` codec: enforces the RFC
	// 1035 63-byte label limit and rejects non-LDH characters. This is what
	// the official Python kubernetes client uses.
	profile := idna.New(
		idna.VerifyDNSLength(true),
		idna.StrictDomainName(true),
	)

	sni := KubeLocalProxySNI(teleportCluster, longKubeCluster)
	_, err := profile.ToASCII(sni)
	require.NoError(t, err,
		"SNI %q must be a valid IDN hostname; strict clients (e.g. the "+
			"official Python kubernetes client) reject it otherwise", sni)
}

func TestKubeLocalProxySNI_DeterministicAndCollisionFree(t *testing.T) {
	const teleportCluster = "example.teleport.sh"

	sni1 := KubeLocalProxySNI(teleportCluster, longKubeCluster)
	sni2 := KubeLocalProxySNI(teleportCluster, longKubeCluster)
	require.Equal(t, sni1, sni2, "KubeLocalProxySNI must be deterministic")

	other := KubeLocalProxySNI(teleportCluster, longKubeCluster+"-other")
	require.NotEqual(t, sni1, other,
		"different kube cluster names must produce different SNIs")
}
