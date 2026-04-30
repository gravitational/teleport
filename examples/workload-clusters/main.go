/*
Copyright 2026 Gravitational, Inc.

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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
)

var (
	// awsAccount is the AWS account to allow tbot to use for joining
	awsAccount = "account"
	// awsARN is the AWS ARN to allow tbot to use for joining
	awsARN = "arn"
	// parentClusterProxyAddress is the parent Teleport Cloud cluster's proxy address
	parentClusterProxyAddress = "parent.teleport.sh"
	// workloadClusterName is the desired name for the new child Teleport Cloud cluster
	workloadClusterName = "company-organization"
)

// TbotConfig defines a configuration for running tbot.
type TbotConfig struct {
	// Version is the configuration version.
	Version string `json:"version"`
	// Oneshot determines if tbot runs as a service.
	Oneshot bool `json:"oneshot"`
	// ProxyServer is the Teleport Proxy to run tbot against.
	ProxyServer string `json:"proxy_server"`
	// Onboarding defines how tbot should attempt to join the Teleport cluster.
	Onboarding Onboarding `json:"onboarding"`
	// Storage instructs tbot where to save its internal certificates.
	Storage Storage `json:"storage"`
	// Services defines which services for tbot to run.
	Services []Service `json:"services"`
}

// Onboarding defines how tbot should attempt to join the Teleport cluster.
type Onboarding struct {
	// JoinMethod is how to join, such as iam.
	JoinMethod string `json:"join_method"`
	// Token is which token in the Teleport cluster to use.
	Token string `json:"token"`
}

// Storage instructs tbot where to save its internal certificates.
type Storage struct {
	// Type is the storage type, such as "memory" for in-memory storage.
	Type string `json:"type"`
}

// Service defines which services for tbot to run.
type Service struct {
	// Type is the service type, such as "identity".
	Type string `json:"type"`
	// Destination is used by the identity service to save retrieved identity files and certs.
	Destination Destination `json:"destination"`
}

// Destination is used by the identity service to save retrieved identity files and certs.
type Destination struct {
	// Type is the type of storage, such as "path".
	Type string `json:"type"`
	// Path is the filepath to use.
	Path string `json:"path"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Failed running workload_cluster lifecycle: %v", err)
	}
}

func run() (err error) {
	ctx := context.Background()

	/**********************************************
	* Create a Teleport workload_cluster resource *
	**********************************************/

	// parentClient is a Teleport client connected to the parent cluster, which
	// assumes the identity used has access for creating, reading, and deleting
	// workload_cluster resources.
	parentClient, err := client.New(ctx, client.Config{
		Addrs: []string{
			parentClusterProxyAddress,
			// Note: port is optional.
		},
		Credentials: []client.Credentials{
			// this loads the credential from tsh
			client.LoadProfile("", ""),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer parentClient.Close()

	// wc defines a workload_cluster to create in us-west-2 with a bot named
	// example-iam and configuration for a token to use IAM joining.
	// The created child Teleport Cloud cluster will have a bot, role, and token
	// each named example-iam. The bot will have access to create, read, and
	// update users and roles.
	wc := &workloadclusterv1.WorkloadCluster{
		Kind:    types.KindWorkloadCluster,
		Version: "v1",
		Metadata: &headerv1.Metadata{
			Name: workloadClusterName,
		},
		Spec: &workloadclusterv1.WorkloadClusterSpec{
			Regions: []*workloadclusterv1.Region{
				{
					Name: "us-west-2",
				},
			},
			Bot: &workloadclusterv1.Bot{
				Name: "example-iam",
			},
			Token: &workloadclusterv1.Token{
				JoinMethod: "iam",
				Allow: []*workloadclusterv1.Allow{
					{
						AwsAccount: awsAccount,
						AwsArn:     awsARN,
					},
				},
			},
		},
	}

	// Create a workload_cluster resource within the parent Teleport Cloud cluster.
	if _, err := parentClient.CreateWorkloadCluster(ctx, wc); err != nil {
		return fmt.Errorf("failed to create workload cluster: %w", err)
	}

	defer func() {
		/****************************
		* Delete a workload cluster *
		****************************/

		// Delete the workload cluster if any error is encountered or if whole lifecycle
		// completes successfully.

		// Clean up the previously created workload_cluster resource in the parent
		// Teleport Cloud cluster.
		if deleteErr := parentClient.DeleteWorkloadCluster(ctx, wc.Metadata.Name); deleteErr != nil {
			err = errors.Join(err, fmt.Errorf("error deleting workload cluster: %w", deleteErr))
		}
	}()

	// Wait for the created workload cluster to reach an active state.
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	wc, err = waitForActiveWorkloadCluster(timeoutCtx, parentClient, wc.Metadata.Name, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed waiting for workload cluster to be active: %w", err)
	}

	/************************************************
	* Run tbot against child Teleport Cloud cluster *
	************************************************/

	// Create a directory that will be used for tbot's configuration and saving a
	// retrieved identity file for interacting with the child Teleport Cloud cluster.
	tbotDir, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("error creating directory for tbot: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tbotDir); err != nil {
			log.Printf("Error removing tbot directory %s: %v", tbotDir, err)
		}
	}()

	// Create the tbot configuration.
	tbotConfig := TbotConfig{
		// Version must be v2.
		Version: "v2",
		// Oneshot should be true to avoid running tbot as a daemon.
		Oneshot: true,
		// ProxyServer should be the Proxy Server including the port 443 for the
		// new child Teleport Cloud cluster.
		ProxyServer: fmt.Sprintf("%s:443", wc.Status.Domain),
		Onboarding: Onboarding{
			// Only iam join method will be supported in the short term for workload_clusters.
			JoinMethod: "iam",
			// Token must match the same name provided in the workload_cluster's
			// Spec.Bot.Name.
			Token: "example-iam",
		},
		Storage: Storage{
			// Configure tbot to use in-memory storage.
			Type: "memory",
		},
		Services: []Service{
			{
				Type: "identity",
				Destination: Destination{
					Type: "directory",
					// A file named identity will be created in the provided path.
					// This identity file may be provided to tctl or Teleport clients
					// for interacting with a Teleport cluster.
					Path: tbotDir,
				},
			},
		},
	}

	// Write the tbot configuration to a `tbot.json` file.
	tbotConfigContent, err := json.Marshal(tbotConfig)
	if err != nil {
		return fmt.Errorf("error marshalling tbot configuration: %w", err)
	}
	tbotConfigPath := filepath.Join(tbotDir, "tbot.json")
	if err := os.WriteFile(tbotConfigPath, tbotConfigContent, 0600); err != nil {
		return fmt.Errorf("error writing tbot configuration: %w", err)
	}

	// Run the tbot binary. Teleport does not expose programmatic access to
	// tbot, so the binary must be used.
	// Once tbot start has successfully completed then an identity file
	// will be populated at the provided path in the tbot configuration.
	var bufErr bytes.Buffer
	tbotCmd := exec.Command("tbot", "start", "-c", tbotConfigPath)
	tbotCmd.Stderr = &bufErr
	if err := tbotCmd.Run(); err != nil {
		return fmt.Errorf("error running tbot: %w\n\n%s", err, bufErr.String())
	}

	/*************************************************************
	* Manage roles and users in the child Teleport Cloud cluster *
	*************************************************************/

	// Create a new Teleport client to interact with the child Teleport Cloud cluster.
	// This client will use the identify file retrieved by tbot.
	childClient, err := client.New(ctx, client.Config{
		Addrs: []string{
			// This is the child Teleport Cloud cluster's proxy address:
			wc.Status.Domain,
			// Note: port is optional.
		},
		Credentials: []client.Credentials{
			// This uses an identity file instead of using a credential from tsh.
			client.LoadIdentityFile(filepath.Join(tbotDir, "identity")),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer childClient.Close()

	// The following section includes examples of creating and deleting
	// a role and a user.

	// Create a new role named example in the child Teleport Cloud cluster.
	newRole := types.RoleV6{
		Metadata: types.Metadata{
			Name: "example",
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					{
						Resources: []string{
							"workload_cluster",
						},
						Verbs: []string{
							"read",
						},
					},
				},
			},
		},
	}
	if _, err := childClient.CreateRole(ctx, &newRole); err != nil {
		return fmt.Errorf("error creating role: %w", err)
	}

	// Create user named "example" that has the new "example" role assigned.
	newUser := types.UserV2{
		Metadata: types.Metadata{
			Name: "example",
		},
		Spec: types.UserSpecV2{
			Roles: []string{
				"example",
			},
		},
	}
	if _, err := childClient.CreateUser(ctx, &newUser); err != nil {
		return fmt.Errorf("error creating user: %w", err)
	}

	// create an invite URL for user to activate account and setup MFA
	resetPasswordToken := proto.CreateResetPasswordTokenRequest{
		Name: newUser.Metadata.Name,
		TTL:  proto.Duration(2 * time.Hour),
		Type: "invite",
	}
	resetToken, err := childClient.CreateResetPasswordToken(ctx, &resetPasswordToken)
	if err != nil {
		return fmt.Errorf("error creating reset token: %w", err)
	}

	ttl := resetToken.Expiry().Sub(time.Now().UTC())
	fmt.Printf("User %q has been created but requires a password. Share this URL with the user to complete user setup, link is valid for %v:\n%v\n\n", newUser.Metadata.Name, ttl, resetToken.GetURL())

	/****************************
	* Delete a workload cluster *
	****************************/
	// Deferred function above will execute and delete the workload_cluster resource in the parent
	// Teleport Cloud cluster.

	return nil
}

func waitForActiveWorkloadCluster(ctx context.Context, client *client.Client, workloadClusterName string, pollingInterval time.Duration) (*workloadclusterv1.WorkloadCluster, error) {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wc, err := client.GetWorkloadCluster(ctx, workloadClusterName)
			if err != nil {
				return nil, fmt.Errorf("error getting workload cluster: %w", err)
			}

			if wc.Status == nil {
				continue
			}

			if wc.Status.State == "active" {
				return wc, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
