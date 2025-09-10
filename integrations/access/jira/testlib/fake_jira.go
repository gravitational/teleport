/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package testlib

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

	"github.com/gravitational/teleport/integrations/access/jira"
)

type FakeJira struct {
	srv              *httptest.Server
	issues           sync.Map
	newIssues        chan jira.Issue
	newIssueComments chan FakeIssueComment
	issueTransitions chan jira.Issue
	pluginUser       jira.UserDetails
	issueIDCounter   uint64
}

type FakeIssueComment struct {
	IssueID string
	jira.Comment
}

func NewFakeJira(author jira.UserDetails, concurrency int) *FakeJira {
	router := httprouter.New()

	mock := &FakeJira{
		newIssues:        make(chan jira.Issue, concurrency),
		newIssueComments: make(chan FakeIssueComment, concurrency*2),
		issueTransitions: make(chan jira.Issue, concurrency),
		srv:              httptest.NewServer(router),
		pluginUser:       author,
	}

	router.GET("/rest/api/2/myself", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
	})
	router.GET("/rest/api/2/project/PROJ", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		project := jira.Project{
			Key:  "PROJ",
			Name: "The Project",
		}
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(&project)
		panicIf(err)
	})
	router.GET("/rest/api/2/mypermissions", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		permissions := jira.Permissions{
			Permissions: map[string]jira.Permission{
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
		var issueInput jira.IssueInput

		err := json.NewDecoder(r.Body).Decode(&issueInput)
		panicIf(err)

		issue := jira.Issue{
			Fields: jira.IssueFields{
				Summary:     issueInput.Fields.Summary,
				Description: issueInput.Fields.Description,
				Type:        *issueInput.Fields.Type,
			},
			Properties: make(map[string]any),
		}
		if issueInput.Fields.Project != nil {
			issue.Fields.Project = *issueInput.Fields.Project
		}
		for _, property := range issueInput.Properties {
			issue.Properties[property.Key] = property.Value
		}
		issue.Fields.Status = jira.StatusDetails{Name: "Pending"}
		issue.Transitions = []jira.IssueTransition{
			{
				ID: "100001", To: jira.StatusDetails{Name: "Approved"},
			},
			{
				ID: "100002", To: jira.StatusDetails{Name: "Denied"},
			},
			{
				ID: "100003", To: jira.StatusDetails{Name: "Expired"},
			},
		}
		issue = mock.StoreIssue(issue)
		mock.newIssues <- issue

		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(rw).Encode(issue)
		panicIf(err)
	})
	router.GET("/rest/api/2/issue/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		issue, found := mock.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(issue)
		panicIf(err)
	})
	router.GET("/rest/api/2/issue/:id/comment", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		issue, found := mock.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		page := issue.Fields.Comment
		descendingComments := make([]jira.Comment, len(page.Comments))
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

		issue, found := mock.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		var payload jira.CommentInput
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		comment := jira.Comment{Body: payload.Body}
		mock.StoreIssueComment(issue, comment)
		mock.newIssueComments <- FakeIssueComment{IssueID: issue.ID, Comment: comment}

		err = json.NewEncoder(rw).Encode(comment)
		panicIf(err)
	})
	router.POST("/rest/api/2/issue/:id/transitions", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		issue, found := mock.GetIssue(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		var payload jira.IssueTransitionInput
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		switch payload.Transition.ID {
		case "100001":
			mock.TransitionIssue(issue, "Approved", mock.pluginUser)
		case "100002":
			mock.TransitionIssue(issue, "Denied", mock.pluginUser)
		case "100003":
			mock.TransitionIssue(issue, "Expired", mock.pluginUser)
		default:
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusNoContent)
	})
	return mock
}

func (s *FakeJira) URL() string {
	return s.srv.URL
}

func (s *FakeJira) Close() {
	s.srv.Close()
	close(s.newIssues)
	close(s.issueTransitions)
}

func (s *FakeJira) GetAuthor() jira.UserDetails {
	return s.pluginUser
}

func (s *FakeJira) StoreIssue(issue jira.Issue) jira.Issue {
	if issue.ID == "" {
		id := atomic.AddUint64(&s.issueIDCounter, 1)
		issue.ID = fmt.Sprintf("%v", id)
		issue.Key = fmt.Sprintf("ISSUE-%v", id)
	}
	s.issues.Store(issue.ID, issue)
	s.issues.Store(issue.Key, issue)
	return issue
}

func (s *FakeJira) GetIssue(idOrKey string) (jira.Issue, bool) {
	if obj, ok := s.issues.Load(idOrKey); ok {
		return obj.(jira.Issue), true
	}
	return jira.Issue{}, false
}

// TransitionIssue transitions an issue between two states.
func (s *FakeJira) TransitionIssue(issue jira.Issue, status string, author jira.UserDetails) jira.Issue {
	issue.Fields.Status = jira.StatusDetails{Name: status}

	changelog := jira.Changelog{
		Author: author,
		Items: []jira.ChangeDetails{
			{
				FieldType: "jira",
				Field:     "status",
				ToString:  status,
			},
		},
	}
	issue.Fields.Status = jira.StatusDetails{Name: status}
	issue.Changelog.Histories = append([]jira.Changelog{changelog}, issue.Changelog.Histories...)
	issue = s.StoreIssue(issue)
	s.issueTransitions <- issue
	return issue
}

func (s *FakeJira) StoreIssueComment(issue jira.Issue, comment jira.Comment) jira.Issue {
	comments := issue.Fields.Comment.Comments
	newComments := make([]jira.Comment, len(comments), len(comments)+1)
	copy(newComments, comments)
	newComments = append(newComments, comment)
	issue.Fields.Comment.Comments = newComments
	issue.Fields.Comment.MaxResults = len(newComments) + 1 // To avoid infinite loop in page iteration
	return s.StoreIssue(issue)
}

func (s *FakeJira) CheckNewIssue(ctx context.Context) (jira.Issue, error) {
	select {
	case issue := <-s.newIssues:
		return issue, nil
	case <-ctx.Done():
		return jira.Issue{}, trace.Wrap(ctx.Err())
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

func (s *FakeJira) CheckIssueTransition(ctx context.Context) (jira.Issue, error) {
	select {
	case issue := <-s.issueTransitions:
		return issue, nil
	case <-ctx.Done():
		return jira.Issue{}, trace.Wrap(ctx.Err())
	}
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
