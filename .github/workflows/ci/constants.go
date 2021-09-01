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

	// APPROVED is a pull request review status
	APPROVED = "APPROVED"

	// ASSIGNMENTS is the environment variable name that stores
	// which reviewers should be assigned to which authors
	ASSIGNMENTS = "ASSIGNMENTS"

	// TOKEN is the env variable name that stores the Github authentication token
	TOKEN = "GITHUB_TOKEN"

	// COMPLETED is a workflow run status
	COMPLETED = "completed"

	// CHECKWORKFLOW is the name of a workflow
	CHECKWORKFLOW = "Check"

	// SYNCHRONIZE is an event type
	SYNCHRONIZE = "synchronize"

	// ASSIGNED is an event type
	ASSIGNED = "assigned"

	//OPENED is an event type
	OPENED = "opened"

	// REOPENED is an event type
	REOPENED = "reopened"

	// READY is an event type
	READY = "ready_for_review"
)
