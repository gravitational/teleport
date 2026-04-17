	// Wait for the created workload cluster to reach an active state.
	timeoutctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	wc, err = waitForActiveWorkloadCluster(timeoutctx, parentClient, wc.Metadata.Name, 30*time.Second)
	if err != nil {
		log.Panicf("failed waiting for workload cluster to be active: %v", err)
	}
