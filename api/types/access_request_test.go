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

package types

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

func TestAssertAccessRequestImplementsResourceWithLabels(t *testing.T) {
	ar, err := NewAccessRequest("test", "test", "test")
	require.NoError(t, err)
	require.Implements(t, (*ResourceWithLabels)(nil), ar)
}

func TestAccessRequestFilter(t *testing.T) {
	var reqs []AccessRequest
	req1, err := NewAccessRequest("0001", "bob", "test")
	require.NoError(t, err)
	reqs = append(reqs, req1)

	req2, err := NewAccessRequest("0002", "alice", "test")
	require.NoError(t, err)
	reqs = append(reqs, req2)

	req3, err := NewAccessRequest("0003", "alice", "test")
	req3.SetReviews([]AccessReview{{Author: "bob"}})
	require.NoError(t, err)
	reqs = append(reqs, req3)

	req4, err := NewAccessRequest("0004", "alice", "test")
	req4.SetReviews([]AccessReview{{Author: "bob"}})
	require.NoError(t, err)
	req4.SetState(RequestState_APPROVED)
	reqs = append(reqs, req4)

	req5, err := NewAccessRequest("0005", "jan", "test")
	require.NoError(t, err)
	req5.SetState(RequestState_DENIED)
	reqs = append(reqs, req5)

	req6, err := NewAccessRequest("0006", "jan", "test")
	require.NoError(t, err)
	reqs = append(reqs, req6)

	cases := []struct {
		name     string
		filter   AccessRequestFilter
		expected []string
	}{
		{
			name: "user wants their own requests",
			filter: AccessRequestFilter{
				Requester: "alice",
				Scope:     AccessRequestScope_MY_REQUESTS,
			},
			expected: []string{"0002", "0003", "0004"},
		},
		{
			name: "user wants requests they need to review",
			filter: AccessRequestFilter{
				Requester: "bob",
				Scope:     AccessRequestScope_NEEDS_REVIEW,
			},
			expected: []string{"0002", "0006"},
		},
		{
			name: "user wants all requests",
			filter: AccessRequestFilter{
				Requester: "bob",
			},
			expected: []string{"0001", "0002", "0003", "0004", "0005", "0006"},
		},
		{
			name: "user wants requests theyve reviewed",
			filter: AccessRequestFilter{
				Requester: "bob",
				Scope:     AccessRequestScope_REVIEWED,
			},
			expected: []string{"0003", "0004"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ids []string
			for _, req := range reqs {
				if tc.filter.Match(req) {
					ids = append(ids, req.GetName())
				}
			}
			require.Equal(t, tc.expected, ids)
		})
	}
}

func TestValidateAssumeStartTime(t *testing.T) {
	creation := time.Now().UTC()
	const day = 24 * time.Hour

	expiry := creation.Add(12 * day)
	maxAssumeStartDuration := creation.Add(constants.MaxAssumeStartDuration)

	testCases := []struct {
		name      string
		startTime time.Time
		errCheck  require.ErrorAssertionFunc
	}{
		{
			name:      "start time too far in the future",
			startTime: creation.Add(constants.MaxAssumeStartDuration + day),
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, trace.BadParameter("assume start time is too far in the future, latest time allowed is %v",
					maxAssumeStartDuration.Format(time.RFC3339)))
			},
		},
		{
			name:      "expired start time",
			startTime: creation.Add(100 * day),
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("assume start time must be prior to access expiry time at %v",
					expiry.Format(time.RFC3339)))
			},
		},
		{
			name:      "before creation start time",
			startTime: creation.Add(-10 * day),
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("assume start time has to be after %v",
					creation.Format(time.RFC3339)))
			},
		},
		{
			name:      "valid start time",
			startTime: creation.Add(6 * day),
			errCheck:  require.NoError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAssumeStartTime(tc.startTime, expiry, creation)
			tc.errCheck(t, err)
		})
	}
}

func Test_AccessRequest_SetExpiry(t *testing.T) {
	// Access requests expiry is a bit special. It is not handled by the backend because we
	// need to emit audit event before they are expired and deleted from the backend.  To
	// achieve SetExpiry method from the Metadata is overwritten (to not set Metadata.Expires)
	// to set expiry in Spec.ResourceExpiry.
	req, err := NewAccessRequest("test_request_1", "alice", "test_role_1")
	require.NoError(t, err)

	reqV3 := req.(*AccessRequestV3)

	require.Nil(t, reqV3.Metadata.Expires)
	require.Nil(t, reqV3.Spec.ResourceExpiry)

	t1 := time.Now().UTC()
	req.SetExpiry(t1)

	require.Nil(t, reqV3.Metadata.Expires)
	require.NotNil(t, reqV3.Spec.ResourceExpiry)
	require.Equal(t, t1, *reqV3.Spec.ResourceExpiry)
}

func TestAccessRequestV3IsEqual(t *testing.T) {
	newReq := func(t *testing.T) *AccessRequestV3 {
		t.Helper()
		req, err := NewAccessRequest("req-1", "alice", "role-a", "role-b")
		require.NoError(t, err)
		return req.(*AccessRequestV3)
	}

	awsID := func(name string, arns ...string) ResourceAccessID {
		return ResourceAccessID{
			Id: ResourceID{ClusterName: "cluster", Kind: "app", Name: name},
			Constraints: &ResourceConstraints{
				Version: V1,
				Details: &ResourceConstraints_AwsConsole{
					AwsConsole: &AWSConsoleResourceConstraints{RoleArns: arns},
				},
			},
		}
	}

	sshID := func(name string, logins ...string) ResourceAccessID {
		return ResourceAccessID{
			Id: ResourceID{ClusterName: "cluster", Kind: "node", Name: name},
			Constraints: &ResourceConstraints{
				Version: V1,
				Details: &ResourceConstraints_Ssh{
					Ssh: &SSHResourceConstraints{Logins: logins},
				},
			},
		}
	}

	unconstrainedID := func(name string) ResourceAccessID {
		return ResourceAccessID{
			Id: ResourceID{ClusterName: "cluster", Kind: "node", Name: name},
		}
	}

	// mockAccessRequest is a minimal implementation of the AccessRequest
	// interface to exercise the non-*AccessRequestV3 type assertion path.
	type mockAccessRequest struct {
		AccessRequestV3
	}

	tests := []struct {
		name string
		a, b func(t *testing.T) AccessRequest
		want bool
	}{
		{
			name: "both nil interface",
			a: func(t *testing.T) AccessRequest {
				var r *AccessRequestV3
				return r
			},
			b: func(t *testing.T) AccessRequest {
				return nil
			},
			want: true,
		},
		{
			name: "both typed nil",
			a: func(t *testing.T) AccessRequest {
				var r *AccessRequestV3
				return r
			},
			b: func(t *testing.T) AccessRequest {
				var r *AccessRequestV3
				return r
			},
			want: true,
		},
		{
			name: "typed nil vs populated",
			a: func(t *testing.T) AccessRequest {
				var r *AccessRequestV3
				return r
			},
			b:    func(t *testing.T) AccessRequest { return newReq(t) },
			want: false,
		},
		{
			name: "populated vs typed nil",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b: func(t *testing.T) AccessRequest {
				var r *AccessRequestV3
				return r
			},
			want: false,
		},
		{
			name: "identical requests",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b:    func(t *testing.T) AccessRequest { return newReq(t) },
			want: true,
		},
		{
			name: "non-v3 type returns false",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b:    func(t *testing.T) AccessRequest { return &mockAccessRequest{} },
			want: false,
		},
		{
			name: "different user",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.User = "bob"
				return r
			},
			want: false,
		},
		{
			name: "different roles",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.Roles = []string{"role-c"}
				return r
			},
			want: false,
		},
		{
			name: "different state",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				require.NoError(t, r.SetState(RequestState_APPROVED))
				return r
			},
			want: false,
		},
		{
			name: "different request reason",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestReason = "urgent"
				return r
			},
			want: false,
		},
		{
			name: "different metadata labels",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Metadata.Labels = map[string]string{"env": "prod"}
				return r
			},
			want: false,
		},
		{
			name: "revision difference ignored",
			a:    func(t *testing.T) AccessRequest { return newReq(t) },
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Metadata.SetRevision("different-revision")
				return r
			},
			want: true,
		},
		{
			name: "aws console constraints equal",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{awsID("app-1", "arn:aws:iam::123:role/admin")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{awsID("app-1", "arn:aws:iam::123:role/admin")}
				return r
			},
			want: true,
		},
		{
			name: "aws console constraints differ by arn",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{awsID("app-1", "arn:aws:iam::123:role/admin")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{awsID("app-1", "arn:aws:iam::123:role/readonly")}
				return r
			},
			want: false,
		},
		{
			name: "ssh constraints equal",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("node-1", "root", "ubuntu")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("node-1", "root", "ubuntu")}
				return r
			},
			want: true,
		},
		{
			name: "ssh constraints differ by login",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("node-1", "root")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("node-1", "ubuntu")}
				return r
			},
			want: false,
		},
		{
			name: "aws vs ssh constraint type mismatch",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{awsID("res-1", "arn:aws:iam::123:role/admin")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("res-1", "root")}
				return r
			},
			want: false,
		},
		{
			name: "constrained vs unconstrained",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("node-1", "root")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{unconstrainedID("node-1")}
				return r
			},
			want: false,
		},
		{
			name: "both unconstrained",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{unconstrainedID("node-1")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{unconstrainedID("node-1")}
				return r
			},
			want: true,
		},
		{
			name: "different resource access ID count",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("node-1", "root")}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{sshID("node-1", "root"), sshID("node-2", "root")}
				return r
			},
			want: false,
		},
		{
			name: "resource access IDs reversed order",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{
					sshID("node-1", "root"),
					awsID("app-1", "arn:aws:iam::123:role/admin"),
				}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{
					awsID("app-1", "arn:aws:iam::123:role/admin"),
					sshID("node-1", "root"),
				}
				return r
			},
			want: false,
		},
		{
			name: "resource access IDs shuffled mixed types",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{
					sshID("node-1", "root"),
					awsID("app-1", "arn:aws:iam::123:role/admin"),
					unconstrainedID("node-2"),
				}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{
					unconstrainedID("node-2"),
					sshID("node-1", "root"),
					awsID("app-1", "arn:aws:iam::123:role/admin"),
				}
				return r
			},
			want: false,
		},
		{
			name: "both nil resource access IDs",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = nil
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = nil
				return r
			},
			want: true,
		},
		{
			name: "typed-nil aws console constraints does not panic",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{{
					Id:          ResourceID{ClusterName: "cluster", Kind: "app", Name: "app-1"},
					Constraints: &ResourceConstraints{Version: V1, Details: (*ResourceConstraints_AwsConsole)(nil)},
				}}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{{
					Id:          ResourceID{ClusterName: "cluster", Kind: "app", Name: "app-1"},
					Constraints: &ResourceConstraints{Version: V1, Details: (*ResourceConstraints_AwsConsole)(nil)},
				}}
				return r
			},
			want: true,
		},
		{
			name: "typed-nil ssh constraints does not panic",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{{
					Id:          ResourceID{ClusterName: "cluster", Kind: "node", Name: "node-1"},
					Constraints: &ResourceConstraints{Version: V1, Details: (*ResourceConstraints_Ssh)(nil)},
				}}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{{
					Id:          ResourceID{ClusterName: "cluster", Kind: "node", Name: "node-1"},
					Constraints: &ResourceConstraints{Version: V1, Details: (*ResourceConstraints_Ssh)(nil)},
				}}
				return r
			},
			want: true,
		},
		{
			name: "typed-nil aws constraints vs populated aws constraints",
			a: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{{
					Id:          ResourceID{ClusterName: "cluster", Kind: "app", Name: "app-1"},
					Constraints: &ResourceConstraints{Version: V1, Details: (*ResourceConstraints_AwsConsole)(nil)},
				}}
				return r
			},
			b: func(t *testing.T) AccessRequest {
				r := newReq(t)
				r.Spec.RequestedResourceAccessIDs = []ResourceAccessID{awsID("app-1", "arn:aws:iam::123:role/admin")}
				return r
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.a(t)
			b := tt.b(t)
			require.Equal(t, tt.want, a.IsEqual(b))
		})
	}
}

// TestResourceConstraintsDetailsExhaustive ensures that resourceConstraintsDetailsEqual
// handles all ResourceConstraints.Details oneof variants. If a new variant is added
// to the proto but not resourceConstraintsDetailsEqual, then the test will fail because the
// function returns false for two identical unknown and non-nil Details types.
func TestResourceConstraintsDetailsExhaustive(t *testing.T) {
	var constraints *ResourceConstraints
	for _, w := range constraints.XXX_OneofWrappers() {
		wrapperType := reflect.TypeOf(w).Elem()
		t.Run(wrapperType.Name(), func(t *testing.T) {
			// Create two ResourceConstraints with identical Details for
			// the current OneOf type.
			rcA := populateResourceConstraintsDetails(t, wrapperType, 1)
			rcB := populateResourceConstraintsDetails(t, wrapperType, 1)

			// Validate that they are not nil.
			require.NotNil(t, rcA.Details)
			require.NotNil(t, rcB.Details)

			// Validate that they are equivalent.
			require.True(t, resourceConstraintsDetailsEqual(rcA, rcB))

			// Validate that a populated constraint and a nil constraint are not equal.
			require.False(t, resourceConstraintsDetailsEqual(rcA, &ResourceConstraints{}))

			// Validate that two ResourceConstraints with the same Details type, but
			// different values are not equivalent.
			rcC := populateResourceConstraintsDetails(t, wrapperType, 2)
			require.False(t, resourceConstraintsDetailsEqual(rcA, rcC))
		})
	}
}

// populateResourceConstraintsDetails returns a ResourceConstraints that has the Details
// field set to a fully populated OneOf variant based on the provided seed. The value
// returned will always be identical for the same type and seed.
func populateResourceConstraintsDetails(t *testing.T, detailsType reflect.Type, seed int) *ResourceConstraints {
	t.Helper()

	wrapper := reflect.New(detailsType).Elem()
	for i := range detailsType.NumField() {
		wf := wrapper.Field(i)
		if !wf.CanSet() || strings.HasPrefix(detailsType.Field(i).Name, "XXX_") {
			continue
		}

		require.Equal(t, reflect.Pointer, wf.Kind(), "unexpected non-pointer field %s in %s oneof", detailsType.Field(i).Name, detailsType.Name())

		inner := reflect.New(wf.Type().Elem())
		innerElem := inner.Elem()
		for j := range innerElem.NumField() {
			sf := innerElem.Type().Field(j)
			f := innerElem.Field(j)
			if !f.CanSet() || strings.HasPrefix(sf.Name, "XXX_") {
				continue
			}

			switch f.Kind() {
			case reflect.String:
				f.SetString(fmt.Sprintf("value-%d", seed))
			case reflect.Bool:
				f.SetBool(seed%2 == 0)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				f.SetInt(int64(seed))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				f.SetUint(uint64(seed))
			case reflect.Float32, reflect.Float64:
				f.SetFloat(float64(seed))
			case reflect.Slice:
				elem := reflect.New(f.Type().Elem()).Elem()
				switch elem.Kind() {
				case reflect.String:
					elem.SetString(fmt.Sprintf("value-%d", seed))
				case reflect.Bool:
					elem.SetBool(seed%2 == 0)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					elem.SetInt(int64(seed))
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					elem.SetUint(uint64(seed))
				case reflect.Float32, reflect.Float64:
					elem.SetFloat(float64(seed))
				default:
					t.Fatalf("unhandled slice element type %s in field %s", elem.Kind(), sf.Name)
				}
				f.Set(reflect.Append(f, elem))
			default:
				t.Fatalf("unhandled kind %s for field %s", f.Kind(), sf.Name)
			}
		}
		wf.Set(inner)
	}

	return &ResourceConstraints{Details: wrapper.Addr().Interface().(isResourceConstraints_Details)}
}
