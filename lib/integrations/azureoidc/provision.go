// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package azureoidc

import (
	"bytes"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// createGraphClient creates a new graph client from ambient credentials (Azure CLI credentials cache).
func createGraphClient() (*msgraphsdk.GraphServiceClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an auth provider using the credential
	authProvider, err := auth.NewAzureIdentityAuthenticationProviderWithScopes(credential, []string{
		"https://graph.microsoft.com/.default",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a request adapter using the auth provider
	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a Graph client using request adapter
	return msgraphsdk.NewGraphServiceClient(adapter), nil
}

func getAzureDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(usr.HomeDir, ".azure"), nil
}

type azureCLIProfile struct {
	Subscriptions []azureCLISubscription `json:"subscriptions"`
}

type azureCLISubscription struct {
	TenantID string `json:"tenantID"`
}

// getTenantID infers the Azure tenant ID from the Azure CLI profiles.
func getTenantID() (string, error) {
	azureDir, err := getAzureDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	payload, err := os.ReadFile(filepath.Join(azureDir, "azureProfile.json"))
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Remove UTF-8 BOM
	payload = bytes.TrimPrefix(payload, []byte("\xef\xbb\xbf"))

	var profile azureCLIProfile
	if err := json.Unmarshal(payload, &profile); err != nil {
		return "", trace.Wrap(err)
	}
	if len(profile.Subscriptions) == 0 {
		return "", trace.BadParameter("subscription not found")
	}

	// Users are expected to run this in the Azure Cloud Shell,
	// where they are by default authenticated to only one subscription.
	return profile.Subscriptions[0].TenantID, nil

}
