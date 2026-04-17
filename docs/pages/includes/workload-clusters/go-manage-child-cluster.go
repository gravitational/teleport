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
		log.Panicf("failed to create client: %v", err)
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
		log.Panicf("error creating role: %v", err)
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
		log.Panicf("error creating user: %v", err)
	}

	// create an invite URL for user to activate account and setup MFA
	resetPasswordToken := proto.CreateResetPasswordTokenRequest{
		Name: newUser.Metadata.Name,
		TTL:  proto.Duration(2 * time.Hour),
		Type: "invite",
	}
	resetToken, err := childClient.CreateResetPasswordToken(ctx, &resetPasswordToken)
	if err != nil {
		log.Panicf("error creating reset token: %v", err)
	}

	ttl := resetToken.Expiry().Sub(time.Now().UTC())
	log.Printf("User %q has been created but requires a password. Share this URL with the user to complete user setup, link is valid for %v:\n%v\n\n", newUser.Metadata.Name, ttl, resetToken.GetURL())
