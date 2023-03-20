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

// 1. Calculate MAU (paginating correctly so that it doesn't undercount)
// 2. Add rate limiting so we don't cause throttling
// 3. Add some test coverage

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	SCAN_DURATION = time.Hour * 24 * 30
	INDEX_NAME    = "timesearchV2"
)

func main() {
	params, err := getParams()
	if err != nil {
		log.Fatal(err)
	}

	// create an AWS session using default SDK behavior, i.e. it will interpret
	// the environment and ~/.aws directory just like an AWS CLI tool would:
	session, err := awssession.NewSessionWithOptions(awssession.Options{
		SharedConfigState: awssession.SharedConfigEnable,
	})
	if err != nil {
		log.Fatal(err)
	}

	svc := dynamodb.New(session)
	state := &trackedState{
		ssh:     make(map[string]struct{}),
		kube:    make(map[string]struct{}),
		db:      make(map[string]struct{}),
		app:     make(map[string]struct{}),
		desktop: make(map[string]struct{}),
	}

	for _, date := range daysBetween(params.startDate, params.startDate.Add(SCAN_DURATION)) {
		err := scanDay(svc, params.tableName, date, state)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func scanDay(svc *dynamodb.DynamoDB, tableName string, date string, state *trackedState) error {
	attributes := map[string]interface{}{
		":date": date,
	}

	attributeValues, err := dynamodbattribute.MarshalMap(attributes)
	if err != nil {
		return err
	}

	var paginationKey map[string]*dynamodb.AttributeValue

	for {
		scanOut, err := svc.Query(&dynamodb.QueryInput{
			TableName:                 aws.String(tableName),
			IndexName:                 aws.String(INDEX_NAME),
			KeyConditionExpression:    aws.String("CreatedAtDate = :date"),
			ExpressionAttributeValues: attributeValues,
			FilterExpression:          aws.String(`EventType IN ("session.start", "db.session.start", "app.session.start", "windows.desktop.session.start")`),
			ExclusiveStartKey:         paginationKey,
		})
		if err != nil {
			return err
		}

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
	FieldsMap map[string]interface{}
}

func reduceEvents(rawEvents []map[string]*dynamodb.AttributeValue, state *trackedState) error {
	for _, rawEvent := range rawEvents {
		var event event
		err := dynamodbattribute.UnmarshalMap(rawEvent, &event)
		if err != nil {
			log.Fatal(err)
		}

		user, ok := event.FieldsMap["user"].(string)
		if !ok {
			return fmt.Errorf("user not found in event")
		}

		var set map[string]struct{}
		switch event.EventType {
		case "session.start":
			set = state.ssh

			if _, ok := event.FieldsMap["kubernetes_cluster"]; ok {
				set = state.kube
			}
		case "db.session.start":
			set = state.db
		case "app.session.start":
			set = state.app
		case "windows.desktop.session.start":
			set = state.desktop
		}

		set[user] = struct{}{}
	}

	return nil
}

// daysBetween returns a list of all dates between `start` and `end` in the format `yyyy-mm-dd`.
func daysBetween(start, end time.Time) []string {
	var days []string
	oneDay := time.Hour * time.Duration(24)
	startDay := daysSinceEpoch(start)
	endDay := daysSinceEpoch(end)

	for startDay <= endDay {
		days = append(days, start.Format(time.DateOnly))
		startDay++
		start = start.Add(oneDay)
	}

	return days
}

func daysSinceEpoch(timestamp time.Time) int64 {
	return timestamp.Unix() / (60 * 60 * 24)
}

type trackedState struct {
	ssh     map[string]struct{}
	kube    map[string]struct{}
	db      map[string]struct{}
	app     map[string]struct{}
	desktop map[string]struct{}
}

type params struct {
	tableName string
	startDate time.Time
}

func getParams() (params, error) {
	tableName := os.Getenv("TABLE_NAME")
	startDate := os.Getenv("START_DATE")

	var timestamp time.Time
	var err error
	if startDate == "" {
		timestamp = time.Now().UTC().Add(-SCAN_DURATION)
		return params{}, fmt.Errorf("START_DATE must be set")
	} else {
		timestamp, err = time.Parse(time.DateOnly, startDate)
		if err != nil {
			return params{}, err
		}
	}

	return params{
		tableName: tableName,
		startDate: timestamp,
	}, nil
}
