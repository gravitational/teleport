func main() {
	ctx := context.Background()

	/**********************************************
	* Create a Teleport workload_cluster resource *
	**********************************************/

	// parentClient is a Teleport client connected to the parent cluster, which
	// assumes the identity used has access for creating, reading, and deleting
	// workload_cluster resources.
	parentClient, err := client.New(ctx, client.Config{
		Addrs: []string{
			// This is the parent Teleport Cloud cluster's proxy address, such as:
			"parent.teleport.sh",
			// Note: port is optional.
		},
		Credentials: []client.Credentials{
			// this loads the credential from tsh
			client.LoadProfile("", ""),
		},
	})
	if err != nil {
		log.Panicf("failed to create client: %v", err)
	}
	defer parentClient.Close()

	// wc defines a workload_cluster to create in us-west-2 with a bot named
	// example-iam and configuration for a token to use IAM joining.
	// The created child Teleport Cloud cluster will have a bot, role, and token
	// each named example-iam. The bot will have access to create, read, and
	// update users and roles.
	wc := &workloadcluster.WorkloadCluster{
		Kind:    types.KindWorkloadCluster,
		Version: "v1",
		Metadata: &headerv1.Metadata{
			Name: "example",
		},
		Spec: &workloadcluster.WorkloadClusterSpec{
			Regions: []*workloadcluster.Region{
				{
					Name: "us-west-2",
				},
			},
			Bot: &workloadcluster.Bot{
				Name: "example-iam",
			},
			Token: &workloadcluster.Token{
				JoinMethod: "iam",
				Allow: []*workloadcluster.Allow{
					{
						AwsAccount: "account",
						AwsArn:     "arn",
					},
				},
			},
		},
	}

	// Create a workload_cluster resource within the parent Teleport Cloud cluster.
	if _, err := parentClient.CreateWorkloadCluster(ctx, wc); err != nil {
		log.Panicf("failed to create workload cluster: %v", err)
	}
