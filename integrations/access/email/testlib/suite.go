/*
Copyright 2024 Gravitational, Inc.

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

package testlib

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/email"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

const (
	// sender default message sender
	sender = "noreply@example.com"
	// allRecipient is a recipient for all messages sent
	allRecipient = "all@example.com"
	// mailgunMockPrivateKey private key for mock mailgun
	mailgunMockPrivateKey = "000000"
	// mailgunMockDomain domain for mock mailgun
	mailgunMockDomain = "test.example.com"
	// subjectIDSubstring indicates start of request id
	subjectIDSubstring = "Role Request "
	// newMessageCount number of original emails
	newMessageCount = 3
	// reviewMessageCount nubmer of review emails per thread
	reviewMessageCount = 6
	// resolveMessageCount number of resolve emails per thread
	resolveMessageCount = 3
	// messageCountPerThread number of total messages per thread
	messageCountPerThread = newMessageCount + reviewMessageCount + resolveMessageCount
)

// EmailBaseSuite implements the test suite for the email access plugin.
// As some plugin features require Teleport Enterprise but the plugin code and
// tests live in the Teleport OSS repo, the test suite can be run both from the
// OSS and the enterprise repo.
type EmailBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig   email.Config
	mockMailgun *mockMailgunServer
	raceNumber  int
}

// SetupTest starts a fake Mailgun, generates the plugin configuration, and
// also starts the plugin. It runs for each test.
func (s *EmailBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = runtime.GOMAXPROCS(0)

	s.mockMailgun = newMockMailgunServer(s.raceNumber)
	s.mockMailgun.Start()
	t.Cleanup(s.mockMailgun.Stop)

	var conf email.Config
	conf.Teleport = s.TeleportConfig()
	conf.Mailgun = &email.MailgunConfig{
		PrivateKey: mailgunMockPrivateKey,
		Domain:     mailgunMockDomain,
		APIBase:    s.mockMailgun.GetURL(),
	}
	conf.Delivery.Sender = sender
	conf.RoleToRecipients = map[string][]string{
		types.Wildcard: {allRecipient},
	}

	s.appConfig = conf

	s.startApp()
}

// startApp starts the email plugin, waits for it to become ready and returns.
func (s *EmailBaseSuite) startApp() {
	t := s.T()
	t.Helper()

	app, err := email.NewApp(s.appConfig)
	require.NoError(t, err)
	integration.RunAndWaitReady(t, app)
}

// EmailSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type EmailSuiteOSS struct {
	EmailBaseSuite
}

// EmailSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type EmailSuiteEnterprise struct {
	EmailBaseSuite
}

// SetupTest overrides EmailBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *EmailSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.EmailBaseSuite.SetupTest()
}

// TestNewThreads tests that the plugin starts new email threads when it
// receives a new access request.
func (s *EmailSuiteOSS) TestNewThreads() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test execution: create an access request, add 2 suggested reviewers
	userName := integration.RequesterOSSUserName
	request := s.CreateAccessRequest(ctx, userName, []string{integration.Reviewer1UserName, integration.Reviewer2UserName})

	pluginData := s.checkPluginData(ctx, request.GetName(), func(data email.PluginData) bool {
		return len(data.EmailThreads) > 0
	})
	require.Len(t, pluginData.EmailThreads, 3) // 2 recipients + all@example.com

	// Validate we got all 3 emails and check their content.
	var messages = s.getMessages(ctx, t, 3)

	require.Len(t, messages, 3)

	// Senders
	require.Equal(t, sender, messages[0].Sender)
	require.Equal(t, sender, messages[1].Sender)
	require.Equal(t, sender, messages[2].Sender)

	// Recipients
	expectedRecipients := []string{allRecipient, integration.Reviewer1UserName, integration.Reviewer2UserName}
	actualRecipients := []string{messages[0].Recipient, messages[1].Recipient, messages[2].Recipient}
	sort.Strings(expectedRecipients)
	sort.Strings(actualRecipients)

	require.Equal(t, expectedRecipients, actualRecipients)

	// Subjects
	require.Contains(t, messages[0].Subject, request.GetName())
	require.Contains(t, messages[1].Subject, request.GetName())
	require.Contains(t, messages[2].Subject, request.GetName())

	// Body
	require.Contains(t, messages[0].Body, fmt.Sprintf("User: %v", userName))
	require.Contains(t, messages[1].Body, "Reason: because of")
	require.Contains(t, messages[2].Body, "Status: ⏳ PENDING")
}

// TestApproval tests that when a request is approved, a followup email is sent
// in the existing thread to notify that the request was approved.
func (s *EmailSuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an access request and wait for its emails
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{integration.Reviewer1UserName})

	s.skipMessages(ctx, t, 2)

	// Test execution: approve the access request
	err := s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validate that we got 2 followup emails and check their content.
	messages := s.getMessages(ctx, t, 2)

	recipients := []string{messages[0].Recipient, messages[1].Recipient}

	require.Contains(t, recipients, allRecipient)
	require.Contains(t, recipients, integration.Reviewer1UserName)

	require.Contains(t, messages[0].Body, "Status: ✅ APPROVED (okay)")
}

// TestDenial tests that when a request is denied, a followup email is sent
// in the existing thread to notify that the request was approved.
func (s *EmailSuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an access request and wait for its emails
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{integration.Reviewer1UserName})

	s.skipMessages(ctx, t, 2)

	// Test execution: approve the access request
	err := s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay")
	require.NoError(t, err)

	// Validate that we got 2 followup emails and check their content.
	messages := s.getMessages(ctx, t, 2)

	recipients := []string{messages[0].Recipient, messages[1].Recipient}

	require.Contains(t, recipients, allRecipient)
	require.Contains(t, recipients, integration.Reviewer1UserName)

	require.Contains(t, messages[0].Body, "Status: ❌ DENIED (not okay)")
}

// TestReviewReplies tests that a followup email is sent after the access request
// is reviewed.
func (s *EmailSuiteEnterprise) TestReviewReplies() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an access request and wait for its emails
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{integration.Reviewer1UserName})
	s.checkPluginData(ctx, req.GetName(), func(data email.PluginData) bool {
		return len(data.EmailThreads) > 0
	})

	s.skipMessages(ctx, t, 2)

	// Test execution: submit an access request review.
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	// Validate we received followup emails with the correct content.
	messages := s.getMessages(ctx, t, 2)
	reply := messages[0].Body
	require.Contains(t, reply, integration.Reviewer1UserName+" reviewed the request", "reply must contain a review author")
	require.Contains(t, reply, "Resolution: ✅ APPROVED", "reply must contain a proposed state")
	require.Contains(t, reply, "Reason: okay", "reply must contain a reason")

	// Test execution: submit a second access request review.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	// Validate we received followup emails with the correct content.
	messages = s.getMessages(ctx, t, 2)
	reply = messages[0].Body
	require.Contains(t, reply, integration.Reviewer2UserName+" reviewed the request", "reply must contain a review author")
	require.Contains(t, reply, "Resolution: ❌ DENIED", "reply must contain a proposed state")
	require.Contains(t, reply, "Reason: not okay", "reply must contain a reason")
}

// TestApprovalByReview tests that followup emails are sent when an access
// request reaches its approval threshold.
func (s *EmailSuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an access request and wait for its emails
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{integration.Reviewer1UserName})

	s.skipMessages(ctx, t, 2)

	// Test execution: submit an access request review and validate we received the review emails.
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	messages := s.getMessages(ctx, t, 2)
	require.Contains(t, messages[0].Body, integration.Reviewer1UserName+" reviewed the request", "reply must contain a review author")

	// Test execution: submit a second access request review and validate we received the review emails.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	messages = s.getMessages(ctx, t, 2)
	require.Contains(t, messages[0].Body, integration.Reviewer2UserName+" reviewed the request", "reply must contain a review author")

	// Validate that a final email is sent to notify that the access request
	// reached its approval threshold and got approved.
	messages = s.getMessages(ctx, t, 2)
	require.Contains(t, messages[0].Body, "Status: ✅ APPROVED (finally okay)")
}

// TestDenialByReview tests that followup emails are sent when an access
// request reaches its denial threshold.
func (s *EmailSuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an access request and wait for its emails
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{integration.Reviewer1UserName})

	s.skipMessages(ctx, t, 2)

	// Test execution: submit an access request review and validate we received the review emails.
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	messages := s.getMessages(ctx, t, 2)
	require.Contains(t, messages[0].Body, integration.Reviewer1UserName+" reviewed the request", "reply must contain a review author")

	// Test execution: submit a second access request review and validate we received the review emails.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	messages = s.getMessages(ctx, t, 2)
	require.Contains(t, messages[0].Body, integration.Reviewer2UserName+" reviewed the request", "reply must contain a review author")

	// Validate that a final email is sent to notify that the access request
	// reached its denial threshold and got approved.
	messages = s.getMessages(ctx, t, 2)
	require.Contains(t, messages[0].Body, "Status: ❌ DENIED (finally not okay)")
}

// TestExpiration tests that when a request expires, a followup email is sent
// in the existing thread to notify that the request has expired.
func (s *EmailSuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an access request and wait for its emails
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{integration.Reviewer1UserName})
	s.skipMessages(ctx, t, 2)

	s.checkPluginData(ctx, req.GetName(), func(data email.PluginData) bool {
		return len(data.EmailThreads) > 0
	})

	// Test execution: expire the access request
	err := s.Ruler().DeleteAccessRequest(ctx, req.GetName()) // simulate expiration
	require.NoError(t, err)

	// Validate an email was sent to notify about the access request expiration.
	messages := s.getMessages(ctx, t, 2)
	require.Contains(t, messages[0].Body, "Status: ⌛ EXPIRED")
}

// TestRace validates that the plugin behaves properly and performs all the
// message updates when a lot of access requests are sent and reviewed in a very
// short time frame.
func (s *EmailSuiteEnterprise) TestRace() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	var (
		raceErr     error
		raceErrOnce sync.Once
		msgIDs      sync.Map
		msgCount    int32
		threadIDs   sync.Map
		replyIDs    sync.Map
		resolveIDs  sync.Map
	)
	setRaceErr := func(err error) error {
		raceErrOnce.Do(func() {
			raceErr = err
		})
		return err
	}
	incCounter := func(m *sync.Map, id string) {
		var newCounter int32
		val, _ := m.LoadOrStore(id, &newCounter)
		counterPtr := val.(*int32)
		atomic.AddInt32(counterPtr, 1)
	}

	process := lib.NewProcess(ctx)
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), integration.Requester1UserName, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			req.SetSuggestedReviewers([]string{integration.Reviewer1UserName, integration.Reviewer2UserName})
			if _, err := s.Requester1().CreateAccessRequestV2(ctx, req); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})
	}

	// 3 original messages + 2*3 reviews + 3 resolve
	for i := 0; i < messageCountPerThread*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.mockMailgun.GetMessage(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			if _, loaded := msgIDs.LoadOrStore(msg.ID, struct{}{}); loaded {
				return setRaceErr(trace.Errorf("message %v already stored", msg.ID))
			}
			atomic.AddInt32(&msgCount, 1)

			reqID := s.extractRequestID(msg.Subject)

			// Handle thread creation notifications
			if strings.Contains(msg.Body, "You have a new Role Request") {
				incCounter(&threadIDs, reqID)

				// We must approve message if it's not an all recipient
				if msg.Recipient != allRecipient {
					if err = s.ClientByName(msg.Recipient).SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
						Author:        msg.Recipient,
						ProposedState: types.RequestState_APPROVED,
						Created:       time.Now(),
						Reason:        "okay",
					}); err != nil {
						return setRaceErr(trace.Wrap(err))
					}
				}
			} else if strings.Contains(msg.Body, "reviewed the request") { // Review
				incCounter(&replyIDs, reqID)
			} else if strings.Contains(msg.Body, "has been resolved") { // Resolution
				incCounter(&resolveIDs, reqID)
			}

			return nil
		})
	}

	process.Terminate()
	<-process.Done()
	require.NoError(t, raceErr)

	threadIDs.Range(func(key, value interface{}) bool {
		next := true

		val, loaded := threadIDs.LoadAndDelete(key)
		next = next && assert.True(t, loaded)

		c, ok := val.(*int32)
		require.True(t, ok)
		require.Equal(t, int32(newMessageCount), *c)

		return next
	})

	replyIDs.Range(func(key, value interface{}) bool {
		next := true

		val, loaded := replyIDs.LoadAndDelete(key)
		next = next && assert.True(t, loaded)

		c, ok := val.(*int32)
		require.True(t, ok)
		require.Equal(t, int32(reviewMessageCount), *c)

		return next
	})

	resolveIDs.Range(func(key, value interface{}) bool {
		next := true

		val, loaded := resolveIDs.LoadAndDelete(key)
		next = next && assert.True(t, loaded)

		c, ok := val.(*int32)
		require.True(t, ok)
		require.Equal(t, int32(resolveMessageCount), *c)

		return next
	})

	// Total message count:
	// (3 original threads + 6 review replies + 3 * resolve) * number of processes
	require.Equal(t, int32(messageCountPerThread*s.raceNumber), msgCount)
}
