//go:build antithesis_imports && !antithesis_imports

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package teleport

// This file keeps the Antithesis Go SDK in the root go.mod. The Antithesis
// toolexec wrapper (antithesis-go-toolexec) injects imports of the SDK into
// instrumented packages at build time, so the module must be resolvable even
// though no Teleport code imports it directly. The build constraint can never
// be satisfied, so this file is never compiled, but `go mod tidy` still sees
// these imports and keeps the requirement and go.sum entries.

import (
	_ "github.com/antithesishq/antithesis-sdk-go/assert"
	_ "github.com/antithesishq/antithesis-sdk-go/instrumentation"
)
