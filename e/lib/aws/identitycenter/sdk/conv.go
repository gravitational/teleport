package sdk

// ToPermissionSetMap transforms PermissionSet to PermissionSetMap.
func ToPermissionSetMap(permSets []*PermissionSet) PermissionSetMap {
	out := make(PermissionSetMap)
	for _, p := range permSets {
		out[p.ARN] = p
	}
	return out
}

// ToAccountMap transforms Account to AccountMap.
func ToAccountMap(accounts []*Account) AccountMap {
	out := make(AccountMap)
	for _, a := range accounts {
		out[a.ID] = a
	}
	return out
}

// ToUserMap transforms Users to UserMap.
func ToUserMap(users []*User) UserMap {
	out := make(UserMap)
	for _, u := range users {
		out[u.ID] = u
	}
	return out
}
