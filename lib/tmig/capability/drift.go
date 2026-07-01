// Copyright 2025 Gravitational, Inc.
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

package capability

// DriftProbeResult records a disagreement between the local table and live behavior.
type DriftProbeResult struct {
	Item     string // e.g. "role:App" or "method:tpm"
	Expected bool   // what the table says
	Actual   bool   // what the probe found
}

// DriftProber defines the interface for running a bounded drift check.
// The real implementation (which does create-and-fail attempts) will be
// wired up when the cluster client is available.
type DriftProber interface {
	Probe(version string, current *CapabilitySet) ([]DriftProbeResult, error)
}
