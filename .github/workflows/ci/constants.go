package ci

const (
	// ASSIGN is the subcommand to assign reviewers
	ASSIGN = "assign-reviewers"

	// CHECK is the subcommand to check reviewers
	CHECK = "check-reviewers"

	// CRON is the subcommand to dismiss runs
	CRON = "dismiss-runs"

	// OPEN is a pull request state
	OPEN = "open"

	// GITHUBREPOSITORY is the environment variable
	// that contains the repo owner and name
	GITHUBREPOSITORY = "GITHUB_REPOSITORY"

	// GITHUBEVENTPATH is the env variable that
	// contains the path to the event payload
	GITHUBEVENTPATH = "GITHUB_EVENT_PATH"

	// GITHUBCOMMIT is a string that is contained in the payload
	// of a commit verified by GitHub.
	// Used to verify commit was made by GH.
	GITHUBCOMMIT = "committer GitHub <noreply@github.com>"

	// APPROVED is a pull request review status.
	APPROVED = "APPROVED"

	// ASSIGNMENTS is the environment variable name that stores
	// which reviewers should be assigned to which authors.
	ASSIGNMENTS = "ASSIGNMENTS"

	// TOKEN is the env variable name that stores the Github authentication token
	TOKEN = "GITHUB_TOKEN"

	// COMPLETED is a workflow run status.
	COMPLETED = "completed"

	// CHECKWORKFLOW is the name of a workflow.
	CHECKWORKFLOW = "Check"

	// SYNCHRONIZE is an event type that is triggered when a commit is pushed to an
	// open pull request.
	SYNCHRONIZE = "synchronize"

	// ASSIGNED is an event type that is triggered when a user is
	// assigned to a pull request.
	ASSIGNED = "assigned"

	//OPENED is an event type that is triggered when a pull request is opened.
	OPENED = "opened"

	// REOPENED is an event type event that is when triggered when a pull request
	// is reopened.
	REOPENED = "reopened"

	// READY is an event type that is triggered when a pull request gets
	// pulled out of a draft state.
	READY = "ready_for_review"
)
