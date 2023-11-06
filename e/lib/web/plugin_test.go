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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/lib/web"
)

func TestRegisterProxyWebHandlers(t *testing.T) {
	h := newWebSuite(t).webPlugin.h

	handlerHasPath(t, h, http.MethodGet, "/enterprise/devices")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/authconnectors")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/saml")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/saml/:name")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/saml/:name")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/oidc")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/oidc/:name")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/oidc/:name")

	handlerHasPath(t, h, http.MethodPost, "/webapi/saml/acs")
	handlerHasPath(t, h, http.MethodGet, "/webapi/saml/sso")
	handlerHasPath(t, h, http.MethodPost, "/webapi/saml/login/console")

	handlerHasPath(t, h, http.MethodGet, "/webapi/oidc/login/web")
	handlerHasPath(t, h, http.MethodGet, "/webapi/oidc/callback")
	handlerHasPath(t, h, http.MethodPost, "/webapi/oidc/login/console")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/license/status")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/license")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/accessrequest")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/accessrequest")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/accessrequest/:requestId")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/accessrequest/:requestId")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/accessrequest")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/resourcerequestroles")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/releases")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/plugin")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/plugin")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/plugin/:name")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/plugins/types")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/plugins/callback/:type")

	handlerHasPath(t, h, http.MethodDelete, "/enterprise/cloud/card")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/card")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/cloud/card")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/billing")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/billing-summary")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/nonbillable-summary")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/payments-invoices")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/invoice-settings")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/cloud/address")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/setupintent")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/upgradewindowstart")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/upgradewindowstart")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/survey")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/start")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/verify")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/newcredentials")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/recovery/token/:token")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/codes")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/recovery/codes")

	handlerHasPath(t, h, http.MethodGet, fmt.Sprintf("%s/*unused", saml.IdPRoute))
	handlerHasPath(t, h, http.MethodPost, fmt.Sprintf("%s/*unused", saml.IdPRoute))
}

// handlerHasPath asserts that the handler has the given path with the given method.
func handlerHasPath(t *testing.T, h *web.Handler, method, path string) {
	handle, _, _ := h.Lookup(method, path)
	require.NotNil(t, handle, "method: %s, path: %s not found", method, path)
}
