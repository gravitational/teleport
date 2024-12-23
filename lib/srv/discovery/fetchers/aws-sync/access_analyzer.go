package aws_sync

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/accessanalyzer"
)

const pollInterval = 100 * time.Millisecond
const maxPollTime = 1 * time.Minute

func (a *awsFetcher) fetchPolicyChanges(ctx context.Context, result *Resources) ([]*accessgraphv1alpha.AWSPolicyChange, error) {
	// Initialize the client
	client, err := a.CloudClients.GetAWSIAMAccessAnalyzerClient(ctx, os.Getenv("AWS_ACCESS_ANALYZER_REGION"), a.getAWSOptions()...)
	if err != nil {
		return nil, err
	}
	analyzerArn := os.Getenv("AWS_ACCESS_ANALYZER_ARN")
	input := &accessanalyzer.ListFindingsV2Input{
		AnalyzerArn: &analyzerArn,
	}
	findings, err := client.ListFindingsV2(input)
	if err != nil {
		return nil, err
	}

	// Generate the finding recommendations
	var findingsWithRecs []*accessanalyzer.FindingSummaryV2
	for _, finding := range findings.Findings {
		if *finding.FindingType != "UnusedPermission" || *finding.Status == "RESOLVED" {
			continue
		}
		_, err = client.GenerateFindingRecommendation(&accessanalyzer.GenerateFindingRecommendationInput{
			AnalyzerArn: aws.String(analyzerArn),
			Id:          finding.Id,
		})
		if err != nil {
			continue
		}
		findingsWithRecs = append(findingsWithRecs, finding)
	}

	// Poll the recommendations until they've been successfully generated or max time is reached
	findingRecs := make(map[string]*accessanalyzer.GetFindingRecommendationOutput)
	timeStart := time.Now()
	for {
		for _, finding := range findingsWithRecs {
			if _, ok := findingRecs[*finding.Id]; ok {
				continue
			}
			rec, err := client.GetFindingRecommendation(&accessanalyzer.GetFindingRecommendationInput{
				AnalyzerArn: aws.String(analyzerArn),
				Id:          finding.Id,
			})
			if err != nil {
				time.Sleep(pollInterval)
				continue
			}
			if *rec.Status == "SUCCEEDED" {
				findingRecs[*finding.Id] = rec
			} else {
				time.Sleep(pollInterval)
			}
		}
		if time.Since(timeStart) > maxPollTime {
			break
		}
		if len(findingRecs) == len(findingsWithRecs) {
			break
		}
	}

	// Get the fetched policies and associate them with the recommendations
	policies := make(map[string]*accessgraphv1alpha.AWSPolicyV1)
	for _, policy := range result.Policies {
		policies[policy.Arn] = policy
	}
	var policyChanges []*accessgraphv1alpha.AWSPolicyChange
	for _, rec := range findingRecs {
		policyChange := &accessgraphv1alpha.AWSPolicyChange{
			ResourceArn: *rec.ResourceArn,
		}
		for _, step := range rec.RecommendedSteps {
			unused := step.UnusedPermissionsRecommendedStep
			existingPolicy, ok := policies[*unused.ExistingPolicyId]
			if !ok {
				fmt.Printf("No existing policy found for %s\n", *unused.ExistingPolicyId)
				continue
			}
			existingDoc, err := url.QueryUnescape(string(existingPolicy.PolicyDocument))
			if err != nil {
				fmt.Printf("Could not decode URL-encoded policy document: %v\n", err)
				continue
			}
			newDoc := ""
			if unused.RecommendedPolicy != nil {
				newDoc = *unused.RecommendedPolicy
			}
			fmt.Printf("Recommending policy change from '%s' to '%s'\n", existingDoc, newDoc)
			change := accessgraphv1alpha.PolicyChange{
				PolicyName:     *unused.ExistingPolicyId,
				ExistingPolicy: existingDoc,
				NewDocument:    newDoc,
				Detach:         false,
			}
			if *unused.RecommendedAction == "DETACH_POLICY" {
				change.Detach = true
			}
			policyChange.Changes = append(policyChange.Changes, &change)
		}
		if len(policyChange.Changes) > 0 {
			policyChanges = append(policyChanges, policyChange)
		} else {
			fmt.Printf("Not appending an empty set of policy changes for %s\n", policyChange.ResourceArn)
		}
	}
	fmt.Printf("Returning %d policy changes\n", len(policyChanges))
	return policyChanges, nil
}
