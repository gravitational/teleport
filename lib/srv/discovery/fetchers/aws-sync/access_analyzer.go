package aws_sync

import (
	"context"
	"fmt"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/accessanalyzer"
)

const pollInterval = 100 * time.Millisecond
const maxPollTime = 1 * time.Minute

func (a *awsFetcher) createAccessTasks(ctx context.Context, result *Resources) error {
	// Initialize the client
	client, err := a.CloudClients.GetAWSIAMAccessAnalyzerClient(ctx, "us-east-2", a.getAWSOptions()...)
	if err != nil {
		return err
	}
	analyzerArn := "arn:aws:access-analyzer:us-east-2:278576220453:analyzer/mbrock-test"
	input := &accessanalyzer.ListFindingsV2Input{
		AnalyzerArn: &analyzerArn,
	}
	findings, err := client.ListFindingsV2(input)
	if err != nil {
		return err
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
	var policyUpdates []*usertasksv1.PolicyUpdate
	for _, rec := range findingRecs {
		for _, step := range rec.RecommendedSteps {
			unused := step.UnusedPermissionsRecommendedStep
			existingPolicy, ok := policies[*unused.ExistingPolicyId]
			// TODO (mbrock): Assuming CREATE_POLICY implies an actual modification, but we need to differentiate
			// between a modification and a removal
			if *unused.RecommendedAction == "CREATE_POLICY" && ok {
				existingDoc := string(existingPolicy.PolicyDocument)
				newDoc := *unused.RecommendedPolicy
				fmt.Printf("Recommending policy change from '%s' to '%s'\n", existingDoc, newDoc)
				policyUpdates = append(policyUpdates, &usertasksv1.PolicyUpdate{
					PolicyName:     *rec.ResourceArn,
					PreviousPolicy: existingDoc,
					NewPolicy:      newDoc,
				})
				if err != nil {
					fmt.Printf("Error updating usertask for recommendation: %v\n", err)
				}
			}
		}
	}

	// Upsert the policy updates as a user task
	task := usertasksv1.UserTask{
		Spec: &usertasksv1.UserTaskSpec{
			AccessGraph: &usertasksv1.AccessGraph{
				RiskFactors: []*usertasksv1.RiskFactor{
					{
						Risk: &usertasksv1.RiskFactor_Policy{
							Policy: &usertasksv1.PolicyRiskFactor{
								Updates: policyUpdates,
							},
						},
					},
				},
			},
		},
	}
	_, err = a.AccessPoint.UpsertUserTask(ctx, &task)
	return nil
}
