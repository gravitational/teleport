package oktaapitest

import oktasdk "github.com/okta/okta-sdk-golang/v2/okta"

type Status string

const (
	StatusStaged          Status = "STAGED"
	StatusProvisioned     Status = "PROVISIONED"
	StatusActive          Status = "ACTIVE"
	StatusRecovery        Status = "RECOVERY"
	StatusPasswordExpired Status = "PASSWORD_EXPIRED"
	StatusLockedOut       Status = "LOCKED_OUT"
	StatusSuspended       Status = "SUSPENDED"
	StatusDeprovisioned   Status = "DEPROVISIONED"
)

type UserArgs struct {
	// ID is Internal Okta ID.
	ID string
	// Name which is basically an email.
	Name string
	// Status of the user.
	Status Status
}

func NewAppUser(args UserArgs) *oktasdk.AppUser {
	return &oktasdk.AppUser{
		Credentials: &oktasdk.AppUserCredentials{
			UserName: args.Name,
		},
		Id:      args.ID,
		Status:  string(args.Status),
		Profile: map[string]any{},
	}
}

func NewOrgUser(args UserArgs) *oktasdk.User {
	return &oktasdk.User{
		Id:     args.ID,
		Status: string(args.Status),
		Profile: &oktasdk.UserProfile{
			"login": args.Name,
		},
	}
}

type ApplicationArgs struct {
	// ID is the Okta internal application ID.
	ID string
	// Label is the Okta application display name.
	Label string
	// Status is the application status. Currently, all non-ACTIVE applications are ignored.
	Status Status
	// Links are Okta application links. Teleport creates one app_server for each link.
	Links []AppLink
}

type AppLink struct {
	Name string `mapstructure:"name"`
	Href string `mapstructure:"href"`
}

func NewApplication(args ApplicationArgs) *oktasdk.Application {
	var appLinks []map[string]string
	for _, link := range args.Links {
		appLinks = append(appLinks, map[string]string{
			"name": link.Name,
			"href": link.Href,
		})
	}
	return &oktasdk.Application{
		Id:     args.ID,
		Name:   "test_app_name_" + args.ID,
		Label:  args.Label,
		Status: string(args.Status),
		Links:  map[string]any{"appLinks": appLinks},
	}
}

type GroupArgs struct {
	// ID is the internal Okta group ID.
	ID string
	// Name is the Okta display name of the group.
	Name string
}

func NewGroup(args GroupArgs) *oktasdk.Group {
	return &oktasdk.Group{
		Id: args.ID,
		Profile: &oktasdk.GroupProfile{
			Name: args.Name,
		},
	}
}
