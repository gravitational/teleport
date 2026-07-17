package ui

import "github.com/gravitational/teleport/api/types"

// UserInfo pairs a Teleport username with its display values.
type UserInfo struct {
	Username string             `json:"username"`
	Display  *types.UserDisplay `json:"display,omitempty"`
}

func newUserInfo(username string, displays map[string]types.UserDisplay) UserInfo {
	info := UserInfo{Username: username}
	if display, ok := displays[username]; ok {
		info.Display = &display
	}
	return info
}
