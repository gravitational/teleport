/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	awsrequest "github.com/aws/aws-sdk-go/aws/request"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	scanDuration = time.Hour * 24 * 30
	indexName    = "timesearchV2"
)

func main() {
	params, err := getParams()
	if err != nil {
		log.Fatal(err)
	}

	// sets of unique users for calculating MAU
	state := &trackedState{
		ssh:     make(map[string]struct{}),
		kube:    make(map[string]struct{}),
		db:      make(map[string]struct{}),
		app:     make(map[string]struct{}),
		desktop: make(map[string]struct{}),
	}

	fmt.Println("Gathering data, this may take a moment")

	// Assume a base read capacity of 25 units per second to start off.
	// If this is too high and we encounter throttling that could impede Teleport, it will be adjusted automatically.
	limiter := newAdaptiveRateLimiter(25)

	// Check the package name for one of the boring primitives. If the package
	// path is from BoringCrypto, we know this binary was compiled using
	// `GOEXPERIMENT=boringcrypto`.
	hash := sha256.New()
	useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
	if reflect.TypeOf(hash).Elem().PkgPath() == "crypto/internal/boring" {
		useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	// create an AWS session using default SDK behavior, i.e. it will interpret
	// the environment and ~/.aws directory just like an AWS CLI tool would:
	session, err := awssession.NewSessionWithOptions(awssession.Options{
		SharedConfigState: awssession.SharedConfigEnable,
		Config: aws.Config{
			Retryer:                       limiter,
			Region:                        aws.String(params.awsRegion),
			CredentialsChainVerboseErrors: aws.Bool(true),
			UseFIPSEndpoint:               useFIPSEndpoint,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Reduce internal retry count so throttling errors bubble up to our rate limiter with less delay.
	svc := dynamodb.New(session)

	for _, date := range daysBetween(params.startDate, params.startDate.Add(scanDuration)) {
		err := scanDay(svc, limiter, params.tableName, date, state)
		if err != nil {
			log.Fatal(err)
		}
	}

	startDate := params.startDate.Format(time.DateOnly)
	endDate := params.startDate.Add(scanDuration).Format(time.DateOnly)
	fmt.Printf("Monthly active users by product during the period %v to %v:\n", startDate, endDate)
	displayProductResults("Server Access", state.ssh, params.showUsers)
	displayProductResults("Kubernetes Access", state.kube, params.showUsers)
	displayProductResults("Database Access", state.db, params.showUsers)
	displayProductResults("Application Access", state.app, params.showUsers)
	displayProductResults("Desktop Access", state.desktop, params.showUsers)
}

func displayProductResults(name string, users map[string]struct{}, showUsers bool) {
	fmt.Printf("  %v: %v", name, len(users))
	if showUsers && len(users) > 0 {
		userList := make([]string, 0, len(users))
		for user := range users {
			userList = append(userList, user)
		}

		fmt.Printf(" (%v)", strings.Join(userList, ", "))
	}

	fmt.Print("\n")
}

// scanDay scans a single day of events from the audit log table.
func scanDay(svc *dynamodb.DynamoDB, limiter *adaptiveRateLimiter, tableName string, date string, state *trackedState) error {
	attributes := map[string]interface{}{
		":date": date,
		":e1":   "session.start",
		":e2":   "db.session.start",
		":e3":   "app.session.start",
		":e4":   "windows.desktop.session.start",
		":e5":   "kube.request",
	}

	attributeValues, err := dynamodbattribute.MarshalMap(attributes)
	if err != nil {
		return err
	}

	var paginationKey map[string]*dynamodb.AttributeValue
	pageCount := 1

outer:
	for {
		fmt.Printf("  scanning date %v page %v...\n", date, pageCount)
		scanOut, err := svc.Query(&dynamodb.QueryInput{
			TableName:                 aws.String(tableName),
			IndexName:                 aws.String(indexName),
			KeyConditionExpression:    aws.String("CreatedAtDate = :date"),
			ExpressionAttributeValues: attributeValues,
			FilterExpression:          aws.String("EventType IN (:e1, :e2, :e3, :e4, :e5)"),
			ExclusiveStartKey:         paginationKey,
			ReturnConsumedCapacity:    aws.String(dynamodb.ReturnConsumedCapacityTotal),
			// We limit the number of items returned to the current capacity to minimize any usage spikes
			// that could affect Teleport as RCUs may be consumed for multiple seconds if the response is large, slowing down Teleport significantly.
			Limit: aws.Int64(int64(limiter.currentCapacity())),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == dynamodb.ErrCodeProvisionedThroughputExceededException {
				fmt.Println("  throttled by DynamoDB, adjusting request rate...")
				limiter.reportThrottleError()
				continue outer
			}

			return err
		}

		pageCount++
		limiter.wait(*scanOut.ConsumedCapacity.CapacityUnits)
		err = reduceEvents(scanOut.Items, state)
		if err != nil {
			return err
		}

		paginationKey = scanOut.LastEvaluatedKey
		if paginationKey == nil {
			break
		}
	}

	return nil
}

type event struct {
	EventType string
	FieldsMap struct {
		User              string
		KubernetesCluster *string `dynamodbav:"kubernetes_cluster,omitempty"`
	}
}

// applies a set of scanned raw events onto the tracked state.
func reduceEvents(rawEvents []map[string]*dynamodb.AttributeValue, state *trackedState) error {
	for _, rawEvent := range rawEvents {
		var event event
		err := dynamodbattribute.UnmarshalMap(rawEvent, &event)
		if err != nil {
			log.Fatal(err)
		}

		var set map[string]struct{}
		switch event.EventType {
		case "kube.request":
			set = state.kube
		case "session.start":
			set = state.ssh
			if event.FieldsMap.KubernetesCluster != nil {
				set = state.kube
			}
		case "db.session.start":
			set = state.db
		case "app.session.start":
			set = state.app
		case "windows.desktop.session.start":
			set = state.desktop
		default:
			return errors.New("unexpected event type: " + event.EventType)
		}

		set[event.FieldsMap.User] = struct{}{}
	}

	return nil
}

// daysBetween returns a list of all dates between `start` and `end` in the format `yyyy-mm-dd`.
func daysBetween(start, end time.Time) []string {
	var days []string
	oneDay := time.Hour * time.Duration(24)
	startDay := daysSinceEpoch(start)
	endDay := daysSinceEpoch(end)

	for startDay < endDay {
		days = append(days, start.Format(time.DateOnly))
		startDay++
		start = start.Add(oneDay)
	}

	return days
}

func daysSinceEpoch(timestamp time.Time) int64 {
	return timestamp.Unix() / (60 * 60 * 24)
}

// trackedState is a set of unique users for each protocol.
type trackedState struct {
	ssh     map[string]struct{}
	kube    map[string]struct{}
	db      map[string]struct{}
	app     map[string]struct{}
	desktop map[string]struct{}
}

type params struct {
	tableName string
	awsRegion string
	showUsers bool
	startDate time.Time
}

func getParams() (params, error) {
	tableName := os.Getenv("TABLE_NAME")
	awsRegion := os.Getenv("AWS_REGION")
	showUsersStr := os.Getenv("SHOW_USERS")
	startDate := os.Getenv("START_DATE")

	if showUsersStr == "" {
		showUsersStr = "false"
	}
	showUsers, err := strconv.ParseBool(showUsersStr)
	if err != nil {
		return params{}, err
	}

	if tableName == "" {
		return params{}, errors.New("TABLE_NAME environment variable is required")
	}

	if awsRegion == "" {
		return params{}, errors.New("AWS_REGION environment variable is required")
	}

	var timestamp time.Time
	if startDate == "" {
		timestamp = time.Now().UTC().Add(-scanDuration)
	} else {
		timestamp, err = time.Parse(time.DateOnly, startDate)
		if err != nil {
			return params{}, err
		}
	}

	return params{
		tableName: tableName,
		awsRegion: awsRegion,
		showUsers: showUsers,
		startDate: timestamp,
	}, nil
}

// adaptiveRateLimiter is a rate limiter that dynamically adjusts its request rate based on throttling errors.
// This unusual strategy was chosen since we cannot know how much free read capacity is available.
//
// This rate limiter progressively increases the request rate when it is not throttled for a longer period of time, and decreases it when it is.
//
// This will never cause actual interrupts to Teleport since the AWS SDK there will retry generously to smooth over
// any possible retries caused by us. The important element is that we back off as soon as we notice this which
// allows Teleport to success eventually.
type adaptiveRateLimiter struct {
	permitCapacity float64
	low            float64
	high           float64
}

func (a *adaptiveRateLimiter) reportThrottleError() {
	a.high = a.permitCapacity
	if math.Abs(a.high-a.low)/a.high < 0.05 {
		a.low = a.high / 2
	}

	old := a.permitCapacity
	capacity := math.Abs(a.high-a.low)/2 + a.low
	// A capacity of zero is not valid and results in requests to be rejected.
	if capacity < 1 {
		capacity = 1.0
	}
	a.permitCapacity = capacity
	fmt.Printf("  throttled by DynamoDB. adjusting request rate from %v RCUs to %v RCUs\n", int(old), int(a.permitCapacity))
}

func (a *adaptiveRateLimiter) wait(permits float64) {
	durationToWait := time.Duration(permits / a.permitCapacity * float64(time.Second))
	time.Sleep(durationToWait)

	if rand.N(10) == 0 {
		a.adjustUp()
	}
}

func (a *adaptiveRateLimiter) adjustUp() {
	a.low = a.permitCapacity
	if math.Abs(a.high-a.low)/a.low < 0.05 {
		a.high = a.low * 2
	}

	old := a.permitCapacity
	a.permitCapacity = math.Abs(a.high-a.low)/2 + a.low
	fmt.Printf("  no throttling for a while. adjusting request rate from %v RCUs to %v RCUs\n", int(old), int(a.permitCapacity))
}

func (a *adaptiveRateLimiter) currentCapacity() float64 {
	return a.permitCapacity
}

func (a *adaptiveRateLimiter) RetryRules(r *awsrequest.Request) time.Duration {
	return 0
}

func (a *adaptiveRateLimiter) ShouldRetry(*awsrequest.Request) bool {
	return false
}

func (a *adaptiveRateLimiter) MaxRetries() int {
	return 0
}

func newAdaptiveRateLimiter(permitsPerSecond float64) *adaptiveRateLimiter {
	fmt.Printf("  setting initial read rate to %v RCUs\n", int(permitsPerSecond))
	return &adaptiveRateLimiter{
		permitCapacity: permitsPerSecond,
		high:           250,
	}
}
