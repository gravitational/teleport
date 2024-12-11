package aws_sync

import (
	"context"
)

func (a *awsFetcher) createAccessTasks(ctx context.Context, result *Resources) error {
	/*
		cfg, err := config.LoadDefaultConfig(ctx)
		cfg.Region = "us-east-2"
		if err != nil {
			panic("Unable to load SDK config, " + err.Error())
		}
		client := accessanalyzer.NewFromConfig(cfg)
		analyzerArn := "arn:aws:access-analyzer:us-east-2:278576220453:analyzer/mbrock-test"
		input := &accessanalyzer.ListFindingsV2Input{
			AnalyzerArn: aws.String(analyzerArn),
		}
		findings, err := client.ListFindingsV2(ctx, input)
		if err != nil {
			return err
		}

		for _, finding := range result.Findings {
			if finding.FindingType != "UnusedPermission" {
				continue
			}
			findingJson, err := json.Marshal(finding)
			fmt.Printf("Finding: %s\n", findingJson)

			_, err = client.GenerateFindingRecommendation(ctx, &accessanalyzer.GenerateFindingRecommendationInput{
				AnalyzerArn: aws.String(analyzerArn),
				Id:          finding.Id,
			})
			if err != nil {
				fmt.Printf("Error generating finding recommendation: %v\n", err)
				continue
			}

			findingRec, err := client.GetFindingRecommendation(ctx, &accessanalyzer.GetFindingRecommendationInput{
				AnalyzerArn: aws.String(analyzerArn),
				Id:          finding.Id,
			})
			if err != nil {
				fmt.Printf("Error getting finding recommendation: %v\n", err)
				continue
			}
			findingRecJson, err := json.Marshal(findingRec)
			fmt.Printf("%s\n", findingRecJson)
		}
	*/
	// TODO: Fetch access analyzer recommendations (sample code above)
	// TODO: Create the user task from AWS policies and access analyzer recommendations
	return nil
}
