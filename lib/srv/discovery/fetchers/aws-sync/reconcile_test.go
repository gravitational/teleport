/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package aws_sync

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func TestReconcileResults(t *testing.T) {
	oldUsers, newUsers := generateUsers()
	oldRoles, newRoles := generateRoles()
	oldEC2, newEC2 := generateEC2()

	oldResults := Resources{
		Users:     oldUsers,
		Roles:     oldRoles,
		Instances: oldEC2,
	}
	newResults := Resources{
		Users:     newUsers,
		Roles:     newRoles,
		Instances: newEC2,
	}

	upsert, delete := ReconcileResults(&oldResults, &newResults)

	wantDelete := []*accessgraphv1alpha.AWSResource{
		{
			Resource: &accessgraphv1alpha.AWSResource_Role{
				Role: oldRoles[0],
			},
		},
	}

	wantUpsert := []*accessgraphv1alpha.AWSResource{
		{
			Resource: &accessgraphv1alpha.AWSResource_Role{
				Role: newRoles[0],
			},
		},
		{
			Resource: &accessgraphv1alpha.AWSResource_Role{
				Role: newRoles[1],
			},
		},
		{
			Resource: &accessgraphv1alpha.AWSResource_User{
				User: newUsers[1],
			},
		},
		{
			Resource: &accessgraphv1alpha.AWSResource_User{
				User: newUsers[2],
			},
		},
		{
			Resource: &accessgraphv1alpha.AWSResource_Instance{
				Instance: newEC2[2],
			},
		},
	}

	require.ElementsMatch(t, wantDelete, delete.Resources)
	require.ElementsMatch(t, wantUpsert, upsert.Resources)
}

func generateUsers() (old, new []*accessgraphv1alpha.AWSUserV1) {
	userA := &accessgraphv1alpha.AWSUserV1{
		UserName: "userA",
		Arn:      "arn:userA",
		Tags:     []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	userBOld := &accessgraphv1alpha.AWSUserV1{
		UserName: "userB",
		Arn:      "arn:userB",
		Tags:     []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	userBNew := &accessgraphv1alpha.AWSUserV1{
		UserName: "userB",
		Arn:      "arn:userB",
		Tags:     []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value2")}},
	}
	userC := &accessgraphv1alpha.AWSUserV1{
		UserName: "userC",
		Arn:      "arn:userC",
		Tags:     []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	old = []*accessgraphv1alpha.AWSUserV1{userA, userBOld}
	new = []*accessgraphv1alpha.AWSUserV1{userA, userC, userBNew}
	return
}

func generateRoles() (old, new []*accessgraphv1alpha.AWSRoleV1) {
	roleA := &accessgraphv1alpha.AWSRoleV1{
		Name: "roleA",
		Arn:  "arn:roleA",
		Tags: []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	roleBOld := &accessgraphv1alpha.AWSRoleV1{
		Name: "roleB",
		Arn:  "arn:roleB",
		Tags: []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	roleBNew := &accessgraphv1alpha.AWSRoleV1{
		Name: "roleB",
		Arn:  "arn:roleB",
		Tags: []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value2")}},
	}
	roleC := &accessgraphv1alpha.AWSRoleV1{
		Name: "roleC",
		Arn:  "arn:roleC",
		Tags: []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	old = []*accessgraphv1alpha.AWSRoleV1{roleA, roleBOld}
	new = []*accessgraphv1alpha.AWSRoleV1{roleC, roleBNew}
	return
}

func generateEC2() (old, new []*accessgraphv1alpha.AWSInstanceV1) {
	instanceA := &accessgraphv1alpha.AWSInstanceV1{
		InstanceId: "instanceA",
		Tags:       []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	instanceBOld := &accessgraphv1alpha.AWSInstanceV1{
		InstanceId: "instanceB",
		Tags:       []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	instanceBNew := &accessgraphv1alpha.AWSInstanceV1{
		InstanceId: "instanceB",
		Tags:       []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value2")}},
	}
	instanceC := &accessgraphv1alpha.AWSInstanceV1{
		InstanceId: "instanceC",
		Tags:       []*accessgraphv1alpha.AWSTag{{Key: "key1", Value: wrapperspb.String("value1")}},
	}
	old = []*accessgraphv1alpha.AWSInstanceV1{instanceA, instanceBOld, instanceC}
	new = []*accessgraphv1alpha.AWSInstanceV1{instanceA, instanceC, instanceBNew}
	return
}
