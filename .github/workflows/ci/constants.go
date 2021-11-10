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

package ci

const (
	// AssignSubcommand is the subcommand to assign reviewers
	AssignSubcommand = "assign-reviewers"

	// CheckSubcommand is the subcommand to check reviewers
	CheckSubcommand = "check-reviewers"

	// Dismiss is the subcommand to dismiss runs
	Dismiss = "dismiss-runs"

	// Open is a pull request state
	Open = "open"

	// GithubRepository is the environment variable
	// that contains the repo owner and name
	GithubRepository = "GITHUB_REPOSITORY"

	// GithubEventPath is the env variable that
	// contains the path to the event payload
	GithubEventPath = "GITHUB_EVENT_PATH"

	// GithubCommit is a string that is contained in the payload
	// of a commit verified by GitHub.
	// Used to verify commit was made by GH.
	GithubCommit = "committer GitHub <noreply@github.com>"

	// Approved is a pull request review status.
	Approved = "APPROVED"

	// Commented is a pull request review status. 
	Commented = "COMMENTED"

	// Token is the env variable name that stores the Github authentication token
	Token = "GITHUB_TOKEN"

	// Completed is a workflow run status.
	Completed = "completed"

	// CheckWorkflow is the name of a workflow.
	CheckWorkflow = "Check"

	// Synchronize is an event type that is triggered when a commit is pushed to an
	// open pull request.
	Synchronize = "synchronize"

	// Assigned is an event type that is triggered when a user is
	// assigned to a pull request.
	Assigned = "assigned"

	// Opened is an event type that is triggered when a pull request is opened.
	Opened = "opened"

	// Reopened is an event type event that is triggered when a pull request
	// is reopened.
	Reopened = "reopened"

	// Ready is an event type that is triggered when a pull request gets
	// pulled out of a draft state.
	Ready = "ready_for_review"

	// Submitted is an event type that is triggered when a pull request review is submitted.
	Submitted = "submitted"

	// Created is an event type that is triggered when a pull request review is created.
	Created = "created"

	// AnyAuthor is the symbol used to get reviewers for external contributors/contrtibutors who
	// do not have designated reviewers.
	AnyAuthor = "*"
)

var Reviewers = map[string][]string{
	"adaadb6":              {"russjones", "r0mant"},
	"aelkugia":             {"russjones", "r0mant"},
	"aharic":               {"russjones", "r0mant"},
	"alex-kovoy":           {"russjones", "r0mant"},
	"alexwolfe":            {"russjones", "r0mant"},
	"anadeliafadeev":       {"russjones", "r0mant"},
	"andrewtaylor6432":     {"russjones", "r0mant"},
	"annabambi":            {"russjones", "r0mant"},
	"aqsa-khan03":          {"russjones", "r0mant"},
	"ardeche07":            {"russjones", "r0mant"},
	"atburke":              {"nklaassen", "fspmarshall"},
	"bardiashahali":        {"russjones", "r0mant"},
	"benarent":             {"russjones", "r0mant"},
	"bernardjkim":          {"russjones", "r0mant"},
	"blake-teleport":       {"russjones", "r0mant"},
	"c-styr":               {"russjones", "r0mant"},
	"campanamarco":         {"russjones", "r0mant"},
	"codingllama":          {"rosstimothy", "quinqu"},
	"colinedwardwood":      {"russjones", "r0mant"},
	"corkrean":             {"russjones", "r0mant"},
	"dboslee":              {"russjones", "r0mant"},
	"deliaconstantino":     {"russjones", "r0mant"},
	"ericduong31":          {"russjones", "r0mant"},
	"fspmarshall":          {"rosstimothy", "codingllama"},
	"gabrielcorado":        {"russjones", "r0mant"},
	"gzdunek":              {"russjones", "r0mant"},
	"haidar-osman":         {"russjones", "r0mant"},
	"halyna-y":             {"russjones", "r0mant"},
	"henninggab":           {"russjones", "r0mant"},
	"ibeckermayer":         {"russjones", "r0mant"},
	"joerger":              {"zmb3", "atburke"},
	"jojohabana":           {"russjones", "r0mant"},
	"jon-can":              {"russjones", "r0mant"},
	"justinas":             {"russjones", "r0mant"},
	"kafipayne":            {"russjones", "r0mant"},
	"kapilville":           {"russjones", "r0mant"},
	"kbence":               {"russjones", "r0mant"},
	"kimlisa":              {"russjones", "r0mant"},
	"klizhentas":           {"russjones", "r0mant"},
	"knisbet":              {"russjones", "r0mant"},
	"kontsevoy":            {"russjones", "r0mant"},
	"logand22":             {"russjones", "r0mant"},
	"mcbattirola":          {"russjones", "r0mant"},
	"michaelmcallister":    {"russjones", "r0mant"},
	"mike-battle":          {"russjones", "r0mant"},
	"najiobeid":            {"russjones", "r0mant"},
	"nataliestaud":         {"russjones", "r0mant"},
	"nickbrennan1":         {"russjones", "r0mant"},
	"nicobon5":             {"russjones", "r0mant"},
	"nklaassen":            {"smallinsky", "tcsc"},
	"pierrebeaucamp":       {"russjones", "r0mant"},
	"programmerq":          {"russjones", "r0mant"},
	"pschisa":              {"russjones", "r0mant"},
	"quinqu":               {"timothyb89", "tcsc"},
	"r0mant":               {"smallinsky", "timothyb89"},
	"rcanderson23":         {"russjones", "r0mant"},
	"recruitingthebest":    {"russjones", "r0mant"},
	"rishibarbhaya-design": {"russjones", "r0mant"},
	"rlutsky":              {"russjones", "r0mant"},
	"rmcentush":            {"russjones", "r0mant"},
	"rosstimothy":          {"r0mant", "fspmarshall"},
	"rudream":              {"russjones", "r0mant"},
	"russjones":            {"zmb3", "r0mant"},
	"sandylcruz":           {"russjones", "r0mant"},
	"sarawpatterson":       {"russjones", "r0mant"},
	"sbrowning-teleport":   {"russjones", "r0mant"},
	"smallinsky":           {"joerger", "r0mant"},
	"sshahcodes":           {"russjones", "r0mant"},
	"stevengravy":          {"russjones", "r0mant"},
	"tcsc":                 {"nklaassen", "codingllama"},
	"terraform-teleport":   {"russjones", "r0mant"},
	"timothyb89":           {"codingllama", "xacrimon"},
	"travelton":            {"russjones", "r0mant"},
	"travisgary":           {"russjones", "r0mant"},
	"twakes":               {"russjones", "r0mant"},
	"tyoungirv":            {"russjones", "r0mant"},
	"ulysseskan":           {"russjones", "r0mant"},
	"valien":               {"russjones", "r0mant"},
	"vitorenesduarte":      {"russjones", "r0mant"},
	"wadells":              {"russjones", "r0mant"},
	"webvictim":            {"russjones", "r0mant"},
	"williamloy":           {"russjones", "r0mant"},
	"xacrimon":             {"zmb3", "joerger"},
	"yjperez":              {"russjones", "r0mant"},
	"zmb3":                 {"rosstimothy", "xacrimon"},
	"*":                    {"russjones", "r0mant"},
}
