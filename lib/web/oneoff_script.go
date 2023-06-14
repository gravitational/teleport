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

package web

import (
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// teleportOneOffScript builds a script that:
// - downloads and extracts teleport binary
// - runs `teleport ` with the args defined in the `args` query param
func (h *Handler) teleportOneOffScript(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	httplib.SetScriptHeaders(w.Header())
	arguments := r.URL.Query().Get("args")

	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs: arguments,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := fmt.Fprintln(w, script); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}
