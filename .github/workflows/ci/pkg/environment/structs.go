package environment

/*
   Below are struct definitions used to transform pull request and review
   events (represented as a json object) into Golang structs. The way these objects are
   structured are different, therefore separate structs for each event are needed
   to unmarshal appropiately.
*/

// PushEvent is used for unmarshalling push events
type PushEvent struct {
	Number      int        `json:"number"`
	PullRequest PR         `json:"pull_request"`
	Repository  Repository `json:"repository"`
	CommitSHA   string     `json:"after"`
	BeforeSHA   string     `json:"before"`
}

// PullRequestEvent s used for unmarshalling pull request events
type PullRequestEvent struct {
	Number      int        `json:"number"`
	PullRequest PR         `json:"pull_request"`
	Repository  Repository `json:"repository"`
}

// ReviewEvent contains metadata about the pull request
// review (used for the pull request review event)
type ReviewEvent struct {
	Review      Review      `json:"review"`
	Repository  Repository  `json:"repository"`
	PullRequest PullRequest `json:"pull_request"`
}

// Head contains the commit sha at the head of the pull request
type Head struct {
	SHA        string `json:"sha"`
	BranchName string `json:"ref"`
}

// Review contains information about the pull request review
type Review struct {
	User User `json:"user"`
}

// User contains information about the user
type User struct {
	Login string `json:"login"`
}

// PullRequest contains information about the pull request (used for pull request review event)
type PullRequest struct {
	Author User `json:"user"`
	Number int  `json:"number"`
	Head   Head `json:"head"`
	Base   Base `json:"base"`
}

// Base contains the base branch commit SHA
type Base struct {
	SHA string `json:"sha"`
}

// PR contains information about the pull request (used for the pull request event)
type PR struct {
	User User
	Head Head `json:"head"`
	Base Base `json:"base"`
}

// Repository contains information about the repository
type Repository struct {
	Name  string `json:"name"`
	Owner Owner  `json:"owner"`
}

// Owner contains information about the repository owner
type Owner struct {
	Name string `json:"login"`
}

// action represents the current action
type action struct {
	Action string `json:"action"`
}
