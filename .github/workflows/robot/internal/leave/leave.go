/*
Copyright 2022 Gravitational, Inc.

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

package leave

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

type Config struct {
	Ctx context.Context
	// RipplingToken is the Rippling authentication token.
	RipplingToken string
	// Usernames is a JSON string of employees' Github
	// usernames mapped to their full name.
	Usernames string
}

type leave struct {
	// onLeave is a map of employees who are on leave.
	onLeave map[string]bool
	// usernames is employees' GitHub usernames mapped to their
	// full name.
	usernames map[string]string
}

func New(c *Config) (Leave, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return leave{}, trace.Wrap(err)
	}
	omit, err := getEmployeesOnLeave(c.Ctx, c.RipplingToken)
	if err != nil {
		return leave{}, trace.Wrap(err)
	}
	var employeeUsernames map[string]string
	if err := json.Unmarshal([]byte(c.Usernames), &employeeUsernames); err != nil {
		return leave{}, trace.Wrap(err)
	}
	return leave{onLeave: omit, usernames: employeeUsernames}, nil
}

func (c *Config) CheckAndSetDefaults() error {
	if c.RipplingToken == "" {
		return trace.BadParameter("missing parameter RipplingToken")
	}
	if c.Ctx == nil {
		c.Ctx = context.Background()
	}
	if c.Usernames == "" {
		return trace.BadParameter("missing parameter Usernames")
	}
	return nil
}

type Leave interface {
	ShouldOmit(name string) bool
}

func (l leave) ShouldOmit(username string) bool {
	fullname, ok := l.usernames[username]
	if !ok {
		return false
	}
	if _, ok := l.onLeave[fullname]; ok {
		return true
	}
	return false
}

// getEmployeesOnLeave gets a map of employees who are on leave.
func getEmployeesOnLeave(ctx context.Context, token string) (map[string]bool, error) {
	now := time.Now()

	leaveRequests, err := getLeaveRequests(ctx, now, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	omit := map[string]bool{}
	for _, req := range leaveRequests {
		// Skip over requests that have any zero-equivalent fields.
		if req.StartDate.IsZero() || req.EndDate.IsZero() || req.FullName == "" {
			log.Printf("Skipping over leave request: %+v.\n", req)
			continue
		}
		if req.shouldOmitEmployee(now) {
			omit[req.FullName] = true
		}
	}
	return omit, nil
}

func (r *employeeLeaveRequest) shouldOmitEmployee(date time.Time) bool {
	// Leave is defined as being out for more than
	// two business days.
	if r.businessDayCount() <= 2 {
		return false
	}

	// Pre-leave omit period to be added to the leave range.
	startOmitPeriod := -2

	// Post-leave omit period to be added to the leave range.
	endOmitPeriod := 1

	// If the request starts on a Monday or Tuesday, subtract two
	// more days to account for non-business days.
	if r.StartDate.Weekday() == time.Monday || r.StartDate.Weekday() == time.Tuesday {
		startOmitPeriod -= 2
	}

	// If the leave request end date is a Friday, add two more days
	// to account for non-business days.
	if r.EndDate.Weekday() == time.Friday {
		endOmitPeriod += 2
	}

	// Subtract and add 1 day to the range so the last return statement
	// returns true if today lands on the start or end date of the
	// leave request omit period.
	start := r.StartDate.Time.AddDate(0, 0, startOmitPeriod-1)
	end := r.EndDate.AddDate(0, 0, endOmitPeriod+1)

	return date.After(start) && date.Before(end)
}

// businessDayCount gets the number of business days
// during the leave request.
func (r *employeeLeaveRequest) businessDayCount() int {
	start, end, businessDays := r.StartDate, r.EndDate, 0
	for !start.After(end.Time) {
		if start.Weekday() != time.Saturday && start.Weekday() != time.Sunday {
			businessDays++
		}
		start.Time = start.AddDate(0, 0, 1)
	}
	return businessDays
}

const (
	// layout is the Time format layout
	layout = "2006-01-02"

	// approvedLeaveRequestStatus is the status of an
	// approved leave request.
	approvedLeaveRequestStatus = "APPROVED"
)

// UnmarshalJSON unmarshals a []byte into Time.
// A custom method is necessary because the UnmarshalJSON method
// for time.Time cannot parse the date returned in the Rippling
// response.
func (t *Time) UnmarshalJSON(b []byte) error {
	timeToParse := strings.Trim(string(b), "\"")
	date, err := time.Parse(layout, timeToParse)
	if err != nil {
		return trace.Wrap(err)
	}
	*t = Time{date}
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t.Time))
}

// Time is a wrapper around time.Time.
type Time struct {
	time.Time
}

type employeeLeaveRequest struct {
	// FullName is the employee's full name.
	FullName string `json:"roleName"`
	// StartDate is the start date of the leave request.
	StartDate Time `json:"startDate"`
	// EndDate is the end date of the leave request.
	EndDate Time `json:"endDate"`
}

// getLeaveRequests gets leave requests from the Rippling API and unmarshals the response
// into []*EmployeeLeaveRequest.
func getLeaveRequests(ctx context.Context, now time.Time, token string) ([]*employeeLeaveRequest, error) {
	ripplingUrl := url.URL{
		Scheme: "https",
		Host:   "api.rippling.com",
		Path:   path.Join("platform", "api", "leave_requests"),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ripplingUrl.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	req.URL.RawQuery = getQueryValuesForGetLeaveRequests(now).Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var leaveRequests []*employeeLeaveRequest
	err = json.Unmarshal([]byte(body), &leaveRequests)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return leaveRequests, nil
}

// getQueryValuesForGetLeaveRequests sets and returns query values.
// This is used in conjunction with getLeaveRequests().
func getQueryValuesForGetLeaveRequests(now time.Time) url.Values {
	// Start query 3 days in the past to get leave requests that may
	// have already ended, but still need to omit the employee from
	// reviews.
	// 3 days is needed to account for non-business days plus the 1
	// day post-leave omit period.
	startQuery := now.AddDate(0, 0, -3)
	formattedStart := fmt.Sprintf("%d-%02d-%02d",
		startQuery.Year(), startQuery.Month(), startQuery.Day())

	// End query 4 days in the future to get future leave requests of
	// the reviewers that need to be omitted.
	// 4 days is needed to account for non-business days plus the 2
	// days pre-leave omit period.
	endQuery := now.AddDate(0, 0, 4)
	formattedEnd := fmt.Sprintf("%d-%02d-%02d",
		endQuery.Year(), endQuery.Month(), endQuery.Day())

	// Set query values.
	q := url.Values{}
	q.Add(from, formattedStart)
	q.Add(to, formattedEnd)
	q.Add(status, approvedLeaveRequestStatus)
	return q
}

// Query parameter constants.
const (
	// to is a parameter name to get leave requests until a specified date.
	to = "to"
	// from is a parameter name to get leave requests from a specified date.
	from = "from"
	// status is a parameter name to filter leave requests by status.
	status = "status"
)
