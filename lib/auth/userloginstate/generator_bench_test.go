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

package userloginstate

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

func BenchmarkGenerate(b *testing.B) {
	user, err := types.NewUser("alice")
	if err != nil {
		b.Fatalf("NewUser: %v", err)
	}

	accessList, err := makeAccessList("acl")
	if err != nil {
		b.Fatalf("makeAccessList: %v", err)
	}

	sizes := []int{100, 1_000, 10_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("members=%d", n), func(b *testing.B) {
			svc, backendSvc, err := initGeneratorSvc()
			if err != nil {
				b.Fatalf("initGeneratorSvc: %v", err)
			}
			_, err = backendSvc.UpsertAccessList(b.Context(), accessList)
			if err != nil {
				b.Fatalf("UpsertAccessList: %v", err)
			}

			for i := 0; i < n; i++ {
				member := makeUserMember(accessList.GetName(), fmt.Sprintf("u%d", i))
				_, err = backendSvc.UpsertAccessListMember(b.Context(), member)
				if err != nil {
					b.Fatalf("UpsertAccessListMember: %v", err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				_, err := svc.Generate(b.Context(), user, backendSvc)
				if err != nil {
					b.Fatalf("Generate: %v", err)
				}
			}
		})
	}
}

func makeAccessList(name string) (*accesslist.AccessList, error) {
	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title: "title",
		Audit: accesslist.Audit{
			NextAuditDate: clockwork.NewRealClock().Now().Add(time.Hour * 48),
		},
		Owners: []accesslist.Owner{{Name: ownerUser}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return accessList, nil
}

func makeUserMember(acl, username string) *accesslist.AccessListMember {
	m := &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{
				Name: username,
			},
		},
		Spec: accesslist.AccessListMemberSpec{
			Name:           username,
			AccessList:     acl,
			MembershipKind: accesslist.MembershipKindUser,
			Joined:         clockwork.NewRealClock().Now(),
			AddedBy:        "system",
		},
	}
	return m
}
