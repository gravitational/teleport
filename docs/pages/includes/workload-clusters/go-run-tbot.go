	/************************************************
	* Run tbot against child Teleport Cloud cluster *
	************************************************/

	// Create a directory that will be used for tbot's configuration and saving a
	// retrieved identity file for interacting with the child Teleport Cloud cluster.
	tbotDir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Panicf("error creating directory for tbot: %v", err)
	}
	defer os.RemoveAll(tbotDir)

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
		log.Panicf("error marshalling tbot configuration: %v", err)
	}
	tbotConfigPath := filepath.Join(tbotDir, "tbot.json")
	if err := os.WriteFile(tbotConfigPath, tbotConfigContent, 0600); err != nil {
		log.Panicf("error writing tbot configuration: %v", err)
	}

	// Run the tbot binary. Teleport does not expose programmatic access to
	// tbot, so the binary must be used.
	// Once tbot start has successfully completed then an identity file
	// will be populated at the provided path in the tbot configuration.
	var bufErr bytes.Buffer
	tbotCmd := exec.Command("tbot", "start", "-c", tbotConfigPath)
	tbotCmd.Stderr = &bufErr
	if err := tbotCmd.Run(); err != nil {
		log.Panicf("error running tbot: %v\n\n%s", err, bufErr.String())
	}
