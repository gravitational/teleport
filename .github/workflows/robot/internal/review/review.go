/*
Copyright 2021 Gravitational, Inc.

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

package review

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"

	"github.com/gravitational/trace"
)

// Reviewer is a code reviewer.
type Reviewer struct {
	// Team the reviewer belongs to.
	Team string `json:"team"`
	// Owner is true if the reviewer is a code or docs owner (required for all reviews).
	Owner bool `json:"owner"`
}

// Config holds code reviewer configuration.
type Config struct {
	// Rand is a random number generator. It is not safe for cryptographic
	// operations.
	Rand *rand.Rand

	// CodeReviewers and CodeReviewersOmit is a map of code reviews and code
	// reviewers to omit.
	CodeReviewers     map[string]Reviewer `json:"codeReviewers"`
	CodeReviewersOmit map[string]bool     `json:"codeReviewersOmit"`

	// DocsReviewers and DocsReviewersOmit is a map of docs reviews and docs
	// reviewers to omit.
	DocsReviewers     map[string]Reviewer `json:"docsReviewers"`
	DocsReviewersOmit map[string]bool     `json:"docsReviewersOmit"`

	// Admins are assigned reviews when no others match.
	Admins []string `json:"admins"`
}

// CheckAndSetDefaults checks and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.Rand == nil {
		c.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	if c.CodeReviewers == nil {
		return trace.BadParameter("missing parameter CodeReviewers")
	}
	if c.CodeReviewersOmit == nil {
		return trace.BadParameter("missing parameter CodeReviewersOmit")
	}

	if c.DocsReviewers == nil {
		return trace.BadParameter("missing parameter DocsReviewers")
	}
	if c.DocsReviewersOmit == nil {
		return trace.BadParameter("missing parameter DocsReviewersOmit")
	}

	if c.Admins == nil {
		return trace.BadParameter("missing parameter Admins")
	}

	return nil
}

// Assignments can be used to assign and check code reviewers.
type Assignments struct {
	c *Config
}

// FromString parses JSON formatted configuration and returns assignments.
func FromString(reviewers string) (*Assignments, error) {
	var c Config
	if err := json.Unmarshal([]byte(reviewers), &c); err != nil {
		return nil, trace.Wrap(err)
	}

	r, err := New(&c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return r, nil
}

// New returns new code review assignments.
func New(c *Config) (*Assignments, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Assignments{
		c: c,
	}, nil
}

// IsInternal returns if the author of a PR is internal.
func (r *Assignments) IsInternal(author string) bool {
	_, code := r.c.CodeReviewers[author]
	_, docs := r.c.DocsReviewers[author]
	return code || docs
}

// Get will return a list of code reviewers a given author.
func (r *Assignments) Get(author string, docs bool, code bool) []string {
	var reviewers []string

	switch {
	case docs && code:
		log.Printf("Assign: Found docs and code changes.")
		reviewers = append(reviewers, r.getDocsReviewers(author)...)
		reviewers = append(reviewers, r.getCodeReviewers(author)...)
	case !docs && code:
		log.Printf("Assign: Found code changes.")
		reviewers = append(reviewers, r.getCodeReviewers(author)...)
	case docs && !code:
		log.Printf("Assign: Found docs changes.")
		reviewers = append(reviewers, r.getDocsReviewers(author)...)
	// Strange state, an empty commit? Return admin reviewers.
	case !docs && !code:
		log.Printf("Assign: Found no docs or code changes.")
		reviewers = append(reviewers, r.getAdminReviewers(author)...)
	}

	return reviewers
}

func (r *Assignments) getDocsReviewers(author string) []string {
	setA, setB := getReviewerSets(author, "Core", r.c.DocsReviewers, r.c.DocsReviewersOmit)
	reviewers := append(setA, setB...)

	// If no docs reviewers were assigned, assign admin reviews.
	if len(reviewers) == 0 {
		return r.getAdminReviewers(author)
	}
	return reviewers
}

func (r *Assignments) getCodeReviewers(author string) []string {
	setA, setB := r.getCodeReviewerSets(author)

	return []string{
		setA[r.c.Rand.Intn(len(setA))],
		setB[r.c.Rand.Intn(len(setB))],
	}
}

func (r *Assignments) getAdminReviewers(author string) []string {
	var reviewers []string
	for _, v := range r.c.Admins {
		if v == author {
			continue
		}
		reviewers = append(reviewers, v)
	}
	return reviewers
}

func (r *Assignments) getCodeReviewerSets(author string) ([]string, []string) {
	// Internal non-Core contributors get assigned from the admin reviewer set.
	// Admins will review, triage, and re-assign.
	v, ok := r.c.CodeReviewers[author]
	if !ok || v.Team == "Internal" {
		reviewers := r.getAdminReviewers(author)
		n := len(reviewers) / 2
		return reviewers[:n], reviewers[n:]
	}

	// Cloud gets reviewers assigned from Core.
	team := v.Team
	if v.Team == "Cloud" {
		team = "Core"
	}

	return getReviewerSets(author, team, r.c.CodeReviewers, r.c.CodeReviewersOmit)
}

// CheckExternal requires two admins have approved.
func (r *Assignments) CheckAdmin(author string, reviews map[string]*github.Review, count int) error {
	log.Printf("Check: Checking for %v admin approvals for %v.", count, author)

	reviewers := r.getAdminReviewers(author)

	n := checkN(reviewers, reviews)
	if n >= count {
		return nil
	}
	return trace.BadParameter("at least %v approvals required from %v, only have %v", count, reviewers, n)
}

// CheckInternal will verify if required reviewers have approved. Checks if
// docs and if each set of code reviews have approved. Admin approvals bypass
// all checks.
func (r *Assignments) CheckInternal(author string, reviews map[string]*github.Review, docs bool, code bool) error {
	log.Printf("Check: Found internal author %v.", author)

	// Skip checks if admins have approved.
	if check(r.getAdminReviewers(author), reviews) {
		return nil
	}

	switch {
	case docs && code:
		log.Printf("Check: Found docs and code changes.")
		if err := r.checkDocsReviews(author, reviews); err != nil {
			return trace.Wrap(err)
		}
		if err := r.checkCodeReviews(author, reviews); err != nil {
			return trace.Wrap(err)
		}
	case !docs && code:
		log.Printf("Check: Found code changes.")
		if err := r.checkCodeReviews(author, reviews); err != nil {
			return trace.Wrap(err)
		}
	case docs && !code:
		log.Printf("Check: Found docs changes.")
		if err := r.checkDocsReviews(author, reviews); err != nil {
			return trace.Wrap(err)
		}
	// Strange state, an empty commit? Check admins.
	case !docs && !code:
		log.Printf("Check: Found no docs or code changes.")
		if checkN(r.getAdminReviewers(author), reviews) < 2 {
			return trace.BadParameter("requires two admin approvals")
		}
	}

	return nil
}

func (r *Assignments) checkDocsReviews(author string, reviews map[string]*github.Review) error {
	reviewers := r.getDocsReviewers(author)

	if check(reviewers, reviews) {
		return nil
	}

	return trace.BadParameter("requires at least one approval from %v", reviewers)
}

func (r *Assignments) checkCodeReviews(author string, reviews map[string]*github.Review) error {
	// External code reviews should never hit this path, if they do, fail and
	// return an error.
	v, ok := r.c.CodeReviewers[author]
	if !ok {
		return trace.BadParameter("rejecting checking external review")
	}

	// Cloud and Internal get reviews from the Core team. Other teams do own
	// internal reviews.
	team := v.Team
	if team == "Internal" || team == "Cloud" {
		team = "Core"
	}

	setA, setB := getReviewerSets(author, team, r.c.CodeReviewers, r.c.CodeReviewersOmit)

	// PRs can be approved if you either have multiple code owners that approve
	// or code owner and code reviewer.
	if checkN(setA, reviews) >= 2 {
		return nil
	}
	if check(setA, reviews) && check(setB, reviews) {
		return nil
	}

	return trace.BadParameter("at least one approval required from each set %v %v", setA, setB)
}

func getReviewerSets(author string, team string, reviewers map[string]Reviewer, reviewersOmit map[string]bool) ([]string, []string) {
	var setA []string
	var setB []string

	for k, v := range reviewers {
		// Only assign within a team.
		if v.Team != team {
			continue
		}
		// Skip over reviewers that are marked as omit.
		if _, ok := reviewersOmit[k]; ok {
			continue
		}
		// Skip author, can't assign/review own PR.
		if k == author {
			continue
		}

		if v.Owner {
			setA = append(setA, k)
		} else {
			setB = append(setB, k)
		}
	}

	return setA, setB
}

func check(reviewers []string, reviews map[string]*github.Review) bool {
	return checkN(reviewers, reviews) > 0
}

func checkN(reviewers []string, reviews map[string]*github.Review) int {
	var n int
	for _, reviewer := range reviewers {
		if review, ok := reviews[reviewer]; ok {
			if review.State == approved && review.Author == reviewer {
				n++
			}
		}
	}
	return n
}

const (
	// approved is a code review where the reviewer has approved changes.
	approved = "APPROVED"
	// changesRequested is a code review where the reviewer has requested changes.
	changesRequested = "CHANGES_REQUESTED"
)
