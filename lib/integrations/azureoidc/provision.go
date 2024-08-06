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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/msgraph"
)

// createGraphClient creates a new graph client from ambient credentials (Azure CLI credentials cache).
func createGraphClient() (*msgraph.Client, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := msgraph.NewClient(msgraph.Config{
		TokenProvider: credential,
	})
	return client, trace.Wrap(err)
}

// EnsureAZLogin invokes `az login` and waits for the command to successfully complete.
// In Azure Cloud Shell, this has the effect of retrieving on-behalf-of user credentials
// which we need to read SSO information (see [CreateTAGCacheFile] and ./private.go),
// as well as prompting the user to choose the desired Azure subscription / directory tenant.
func EnsureAZLogin(ctx context.Context) error {
	fmt.Println("We will execute `az login` to acquire the necessary permissions and allow you to choose the desired Entra ID tenant.")
	fmt.Println("Please follow the instructions below.")
	cmd := exec.CommandContext(ctx, "az", "login", "--only-show-errors")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return trace.Wrap(cmd.Run())
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
	TenantID  string `json:"tenantID"`
	IsDefault bool   `json:"isDefault"`
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
		return "", trace.Wrap(err, "failed to parse Azure profile")
	}

	for _, subscription := range profile.Subscriptions {
		if subscription.IsDefault {
			return subscription.TenantID, nil
		}
	}

	return "", trace.NotFound("subscription not found")
}
