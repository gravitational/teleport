package main

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

const (
	username  = "authtestuser1234567"
	groupname = "authtestgroup123456"
)

var (
	utfUsername, _  = windows.UTF16PtrFromString(username)
	utfGroupname, _ = windows.UTF16PtrFromString(groupname)
)

func TestLSA(t *testing.T) {
	setupDispatchTable()
	for name, test := range map[string]func(*testing.T){
		"toLSAString": testToLSAString,
	} {
		t.Run(name, test)
	}
}

func TestCreateGroups(t *testing.T) {
	_, err := user.LookupGroup(groupname)
	require.Error(t, err)
	require.NoError(t, createGroups([]string{groupname}))
	group, err := user.LookupGroup(groupname)
	require.NoError(t, err)
	require.Equal(t, groupname, group.Name)

	require.NoError(t, NetLocalGroupDel(nil, utfGroupname))
}

func TestEnsureUser(t *testing.T) {
	_, err := user.Lookup(username)
	require.Error(t, err)
	require.NoError(t, ensureUser(username))
	u, err := user.Lookup(username)
	require.NoError(t, err)
	require.Equal(t, username, u.Name)
	group, err := user.LookupGroup(teleportUsers)
	require.NoError(t, err)
	ids, err := u.GroupIds()
	require.NoError(t, err)
	require.Contains(t, ids, group.Gid)

	require.NoError(t, ensureUser(username))
	u2, err := user.Lookup(username)
	require.NoError(t, err)
	require.Equal(t, u.Uid, u2.Uid)
	require.NoError(t, NetUserDel(nil, utfUsername))
}

func TestGroupOperations(t *testing.T) {
	require.NoError(t, createGroups([]string{groupname}))
	require.NoError(t, ensureUser(username))
	u, err := user.Lookup(username)
	require.NoError(t, err)
	require.NoError(t, addToGroup(u, groupname))

	group, err := user.LookupGroup(groupname)
	require.NoError(t, err)
	u, err = user.Lookup(username)
	require.NoError(t, err)
	ids, err := u.GroupIds()
	require.NoError(t, err)
	require.Contains(t, ids, group.Gid)

	require.NoError(t, NetUserDel(nil, utfUsername))
	require.NoError(t, NetLocalGroupDel(nil, utfGroupname))
}
