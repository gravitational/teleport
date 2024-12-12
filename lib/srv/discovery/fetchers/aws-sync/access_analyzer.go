package aws_sync

import (
	"context"
	"fmt"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"golang.org/x/sync/errgroup"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/accessanalyzer"
)

const pollInterval = 100 * time.Millisecond
const maxPollTime = 1 * time.Minute

func (a *awsFetcher) fetchPolicyChanges(ctx context.Context, result *Resources) ([]*accessgraphv1alpha.AWSPolicyChange, error) {
	// Initialize the client
	client, err := a.CloudClients.GetAWSIAMAccessAnalyzerClient(ctx, "us-east-2", a.getAWSOptions()...)
	if err != nil {
		return nil, err
	}
	analyzerArn := "arn:aws:access-analyzer:us-east-2:278576220453:analyzer/mbrock-test"
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
	var findingRecs sync.Map
	var findingRecsCount atomic.Int64
	timeStart := time.Now()
	for {
		eg, _ := errgroup.WithContext(ctx)
		for _, finding := range findingsWithRecs {
			if _, ok := findingRecs.Load(*finding.Id); ok {
				continue
			}
			eg.Go(func() error {
				rec, _ := client.GetFindingRecommendation(&accessanalyzer.GetFindingRecommendationInput{
					AnalyzerArn: aws.String(analyzerArn),
					Id:          finding.Id,
				})
				if err != nil {
					return nil
				}
				fmt.Printf("Get recommendation for finding %s: %s", *finding.Id, *rec.Status)
				if *rec.Status == "SUCCEEDED" {
					findingRecs.Store(*finding.Id, rec)
					findingRecsCount.Add(1)
				}
				return nil
			})
		}
		_ = eg.Wait()
		if time.Since(timeStart) > maxPollTime {
			break
		}
		if findingRecsCount.Load() == int64(len(findingsWithRecs)) {
			break
		}
		// Sleep before retrying the poll
		time.Sleep(pollInterval)
	}

	// Get the fetched policies and associate them with the recommendations
	policies := make(map[string]*accessgraphv1alpha.AWSPolicyV1)
	for _, policy := range result.Policies {
		policies[policy.Arn] = policy
	}
	var policyChanges []*accessgraphv1alpha.AWSPolicyChange
	findingRecs.Range(func(k, v interface{}) bool {
		rec := v.(*accessanalyzer.GetFindingRecommendationOutput)
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
				PolicyName:     existingPolicy.PolicyName,
				PolicyArn:      *unused.ExistingPolicyId,
				ExistingPolicy: existingDoc,
				NewDocument:    newDoc,
				Detach:         false,
			}
			// TODO (mbrock): Filtering out detach recommendations for now
			if *unused.RecommendedAction != "DETACH_POLICY" {
				policyChange.Changes = append(policyChange.Changes, &change)
			}
		}
		if len(policyChange.Changes) > 0 {
			policyChanges = append(policyChanges, policyChange)
		} else {
			fmt.Printf("Not appending an empty set of policy changes for %s\n", policyChange.ResourceArn)
		}
		return true
	})
	fmt.Printf("Returning %d policy changes\n", len(policyChanges))
	return policyChanges, nil
}
