/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package accesslist

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

func TestAccessListMemberDefaults(t *testing.T) {
	newValidAccessListMember := func() *AccessListMember {
		accesslist := uuid.New().String()
		user := "some-user"

		return &AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Kind:    types.KindAccessListMember,
				Version: types.V1,
				Metadata: header.Metadata{
					Name: fmt.Sprintf("%s/%s", accesslist, user),
				},
			},
			Spec: AccessListMemberSpec{
				AccessList: accesslist,
				Name:       user,
				Joined:     time.Date(1969, time.July, 20, 20, 17, 40, 0, time.UTC),
				AddedBy:    "some-other-user",
			},
		}
	}

	t.Run("join date required for member", func(t *testing.T) {
		uut := newValidAccessListMember()
		uut.Spec.Joined = time.Time{}

		err := uut.CheckAndSetDefaults()
		require.Error(t, err)
	})

	t.Run("added-by required", func(t *testing.T) {
		uut := newValidAccessListMember()
		uut.Spec.AddedBy = ""

		err := uut.CheckAndSetDefaults()
		require.Error(t, err)
	})
}
