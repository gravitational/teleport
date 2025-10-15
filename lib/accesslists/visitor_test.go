/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package accesslists

import (
	"strconv"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

func generateAccessList(name string) *accesslist.AccessList {
	return &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{
				Name: name,
			},
		},
	}
}

func generateNestedALs(level, directMembers int, rootListName, userName string) (map[string]*accesslist.AccessList, map[string][]*accesslist.AccessListMember) {
	accesslists := []*accesslist.AccessList{generateAccessList(rootListName)}
	members := make(map[string][]*accesslist.AccessListMember)

	for i := range level - 1 {
		parentName := accesslists[i].GetName()
		name := "nested-al-" + strconv.Itoa(i)
		accesslists = append(accesslists, generateAccessList(name))
		listMembers := generateUserMembers(directMembers/2, name)
		listMembers = append(listMembers, &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: name,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     parentName,
				Name:           name,
				MembershipKind: accesslist.MembershipKindList,
			},
		})
		listMembers = append(listMembers, generateUserMembers(directMembers/2+directMembers%2, name)...)
		members[parentName] = listMembers
	}

	alMap := make(map[string]*accesslist.AccessList)
	for _, al := range accesslists {
		alMap[al.GetName()] = al
	}
	return alMap, members
}

func generateUserMembers(count int, alName string) []*accesslist.AccessListMember {
	var members []*accesslist.AccessListMember
	for i := range count {
		memberName := "member-" + strconv.Itoa(i)
		members = append(members, &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: memberName,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     alName,
				Name:           memberName,
				MembershipKind: accesslist.MembershipKindUser,
			},
		})
	}
	return members
}

func BenchmarkIsAccessListMember(b *testing.B) {
	const mainAccessListName = "main-al"
	const testUserName = "test-user"

	lockGetter := &mockLocksGetter{}
	clock := clockwork.NewFakeClock()

	b.Run("no accessPaths", func(b *testing.B) {
		mock := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				mainAccessListName: generateAccessList(mainAccessListName),
			},
			members: map[string][]*accesslist.AccessListMember{
				mainAccessListName: {},
			},
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("single-page direct member", func(b *testing.B) {
		member := &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: testUserName,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     mainAccessListName,
				Name:           testUserName,
				MembershipKind: accesslist.MembershipKindUser,
			},
		}
		generatedMembers := generateUserMembers(50, mainAccessListName)
		// We inject the member we are looking for in the middle of the member list
		members := append(generatedMembers[:25], member)
		members = append(members, generatedMembers[25:]...)

		mock := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				mainAccessListName: generateAccessList(mainAccessListName),
			},
			members: map[string][]*accesslist.AccessListMember{
				mainAccessListName: members,
			},
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("multiple-pages direct member", func(b *testing.B) {
		member := &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name: testUserName,
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList:     mainAccessListName,
				Name:           testUserName,
				MembershipKind: accesslist.MembershipKindUser,
			},
		}
		generatedMembers := generateUserMembers(500, mainAccessListName)
		// We inject the member we are looking for in the middle of the member list
		members := append(generatedMembers[:250], member)
		members = append(members, generatedMembers[250:]...)

		mock := &mockAccessListAndMembersGetter{
			accessLists: map[string]*accesslist.AccessList{
				mainAccessListName: generateAccessList(mainAccessListName),
			},
			members: map[string][]*accesslist.AccessListMember{
				mainAccessListName: members,
			},
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("single-page nested member", func(b *testing.B) {
		lists, members := generateNestedALs(5, 0, mainAccessListName, testUserName)
		mock := &mockAccessListAndMembersGetter{
			accessLists: lists,
			members:     members,
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("multiple pages nested member", func(b *testing.B) {
		lists, members := generateNestedALs(5, 501, mainAccessListName, testUserName)
		mock := &mockAccessListAndMembersGetter{
			accessLists: lists,
			members:     members,
		}

		for b.Loop() {
			_, err := IsAccessListMember(
				b.Context(),
				&types.UserV2{Metadata: types.Metadata{Name: testUserName}},
				generateAccessList(mainAccessListName),
				mock,
				lockGetter,
				clock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
