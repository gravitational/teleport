/*
Copyright 2020-2021 Gravitational, Inc.

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

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

type FakeJira struct {
	srv              *httptest.Server
	issues           sync.Map
	newIssues        chan Issue
	newIssueComments chan FakeIssueComment
	issueTransitions chan Issue
	author           UserDetails
	issueIDCounter   uint64
}

type FakeIssueComment struct {
	IssueID string
	Comment
}

func NewFakeJira(author UserDetails, concurrency int) *FakeJira {
	router := httprouter.New()

	self := &FakeJira{
		newIssues:        make(chan Issue, concurrency),
		newIssueComments: make(chan FakeIssueComment, concurrency*2),
		issueTransitions: make(chan Issue, concurrency),
		srv:              httptest.NewServer(router),
		author:           author,
	}

	router.GET("/rest/api/2/myself", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
	})
	router.GET("/rest/api/2/project/PROJ", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		project := Project{
			Key:  "PROJ",
			Name: "The Project",
		}
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(&project)
		panicIf(err)
	})
	router.GET("/rest/api/2/mypermissions", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		permissions := Permissions{
			Permissions: map[string]Permission{
				"BROWSE_PROJECTS": {
					HavePermission: true,
				},
				"CREATE_ISSUES": {
					HavePermission: true,
				},
				"TRANSITION_ISSUES": {
					HavePermission: true,
				},
				"ADD_COMMENTS": {
					HavePermission: true,
				},
			},
		}
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(&permissions)
		panicIf(err)
	})
	router.POST("/rest/api/2/issue", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var issueInput IssueInput

		err := json.NewDecoder(r.Body).Decode(&issueInput)
		panicIf(err)

		issue := Issue{
			Fields: IssueFields{
				Summary:     issueInput.Fields.Summary,
				Description: issueInput.Fields.Description,
			},
			Properties: make(map[string]interface{}),
		}
		if issueInput.Fields.Project != nil {
			issue.Fields.Project = *issueInput.Fields.Project
		}
		for _, property := range issueInput.Properties {
			issue.Properties[property.Key] = property.Value
		}
		issue.Fields.Status = StatusDetails{Name: "Pending"}
		issue.Transitions = []IssueTransition{
			{
				ID: "100001", To: StatusDetails{Name: "Approved"},
			},
			{
				ID: "100002", To: StatusDetails{Name: "Denied"},
			},
			{
				ID: "100003", To: StatusDetails{Name: "Expired"},
			},
		}
		issue = self.StoreIssue(issue)
		self.newIssues <- issue

		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(rw).Encode(issue)
		panicIf(err)
	})
	router.GET("/rest/api/2/issue/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		issue, found := self.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(issue)
		panicIf(err)
	})
	router.GET("/rest/api/2/issue/:id/comment", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		issue, found := self.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		page := issue.Fields.Comment
		descendingComments := make([]Comment, len(page.Comments))
		for i, comment := range page.Comments {
			descendingComments[len(page.Comments)-i-1] = comment
		}
		page.Comments = descendingComments

		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(page)
		panicIf(err)
	})
	router.POST("/rest/api/2/issue/:id/comment", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		issue, found := self.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		var payload CommentInput
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		comment := Comment{Body: payload.Body}
		self.StoreIssueComment(issue, comment)
		self.newIssueComments <- FakeIssueComment{IssueID: issue.ID, Comment: comment}

		err = json.NewEncoder(rw).Encode(comment)
		panicIf(err)
	})
	router.POST("/rest/api/2/issue/:id/transitions", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		issue, found := self.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		var payload IssueTransitionInput
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		switch payload.Transition.ID {
		case "100001":
			self.TransitionIssue(issue, "Approved")
		case "100002":
			self.TransitionIssue(issue, "Denied")
		case "100003":
			self.TransitionIssue(issue, "Expired")
		default:
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusNoContent)
	})
	return self
}

func (s *FakeJira) URL() string {
	return s.srv.URL
}

func (s *FakeJira) Close() {
	s.srv.Close()
	close(s.newIssues)
	close(s.issueTransitions)
}

func (s *FakeJira) GetAuthor() UserDetails {
	return s.author
}

func (s *FakeJira) StoreIssue(issue Issue) Issue {
	if issue.ID == "" {
		id := atomic.AddUint64(&s.issueIDCounter, 1)
		issue.ID = fmt.Sprintf("%v", id)
		issue.Key = fmt.Sprintf("ISSUE-%v", id)
	}
	s.issues.Store(issue.ID, issue)
	s.issues.Store(issue.Key, issue)
	return issue
}

func (s *FakeJira) GetIssue(idOrKey string) (Issue, bool) {
	if obj, ok := s.issues.Load(idOrKey); ok {
		return obj.(Issue), true
	}
	return Issue{}, false
}

func (s *FakeJira) TransitionIssue(issue Issue, status string) Issue {
	issue.Fields.Status = StatusDetails{Name: status}

	changelog := Changelog{
		Author: s.author,
		Items: []ChangeDetails{
			{
				FieldType: "jira",
				Field:     "status",
				ToString:  status,
			},
		},
	}
	issue.Changelog.Histories = append([]Changelog{changelog}, issue.Changelog.Histories...)
	issue = s.StoreIssue(issue)
	s.issueTransitions <- issue
	return issue
}

func (s *FakeJira) StoreIssueComment(issue Issue, comment Comment) Issue {
	comments := issue.Fields.Comment.Comments
	newComments := make([]Comment, len(comments), len(comments)+1)
	copy(newComments, comments)
	newComments = append(newComments, comment)
	issue.Fields.Comment.Comments = newComments
	issue.Fields.Comment.MaxResults = len(newComments) + 1 // To avoid infinite loop in page iteration
	return s.StoreIssue(issue)
}

func (s *FakeJira) CheckNewIssue(ctx context.Context) (Issue, error) {
	select {
	case issue := <-s.newIssues:
		return issue, nil
	case <-ctx.Done():
		return Issue{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeJira) CheckNewIssueComment(ctx context.Context) (FakeIssueComment, error) {
	select {
	case comment := <-s.newIssueComments:
		return comment, nil
	case <-ctx.Done():
		return FakeIssueComment{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeJira) CheckIssueTransition(ctx context.Context) (Issue, error) {
	select {
	case issue := <-s.issueTransitions:
		return issue, nil
	case <-ctx.Done():
		return Issue{}, trace.Wrap(ctx.Err())
	}
}

func panicIf(err error) {
	if err != nil {
		log.Panicf("%v at %v", err, string(debug.Stack()))
	}
}
