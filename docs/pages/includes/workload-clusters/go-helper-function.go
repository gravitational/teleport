func waitForActiveWorkloadCluster(ctx context.Context, client *client.Client, workloadClusterName string, pollingInterval time.Duration) (*workloadcluster.WorkloadCluster, error) {
	for {
		timer := time.NewTimer(pollingInterval)

		select {
		case <-timer.C:
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
