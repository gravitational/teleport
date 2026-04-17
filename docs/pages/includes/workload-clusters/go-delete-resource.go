	/****************************
	* Delete a workload cluster *
	****************************/

	// Cleanup the previously created workload_cluster resource in the parent
	// Teleport Cloud cluster.
	if err := parentClient.DeleteWorkloadCluster(ctx, wc.Metadata.Name); err != nil {
		log.Panicf("error deleting cloud cluster: %v", err)
	}
}
