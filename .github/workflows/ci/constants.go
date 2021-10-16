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

	// Signature is a prefix used for the commit signature file name.
	Signature = "signature"

	// Payload is a prefix used for the commit payload file name.
	Payload = "payload"
	// GithubKey is a prefix used for the Github web-flow public key file name.
	GithubKey = "github-key"
	// WebflowKeyURL is the URL that points to Github's public GPG key.
	// GitHub sets the committer for all commits made using their web
	// interface to the user "web-flow".
	WebflowKeyURL = "https://github.com/web-flow.gpg"
)
