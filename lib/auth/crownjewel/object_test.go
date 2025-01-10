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

package crownjewel_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/lib/auth/crownjewel"
)

func TestValidateCrownJewel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		jewel   *crownjewelv1.CrownJewel
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "NilCrownJewel",
			jewel:   nil,
			wantErr: require.Error,
		},
		{
			name: "ValidCrownJewel",
			jewel: &crownjewelv1.CrownJewel{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &crownjewelv1.CrownJewelSpec{
					TeleportMatchers: []*crownjewelv1.TeleportMatcher{
						{
							Kinds: []string{"db"},
							Names: []string{"name1"},
							Labels: []*labelv1.Label{
								{
									Name:   "label1",
									Values: []string{"value1"},
								},
							},
						},
					},
					AwsMatchers: []*crownjewelv1.AWSMatcher{
						{
							Types: []string{"type1"},
							Arns:  []string{"arn1"},
							Tags: []*crownjewelv1.AWSTag{
								{
									Key:    "key1",
									Values: []*wrapperspb.StringValue{wrapperspb.String("value1")},
								},
							},
						},
					},
				},
			},
			wantErr: require.NoError,
		},
		{
			name: "ValidCrownJewelWithQuery",
			jewel: &crownjewelv1.CrownJewel{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &crownjewelv1.CrownJewelSpec{
					Query: "SELECT * FROM nodes",
				},
			},
			wantErr: require.NoError,
		},
		{
			name: "MissingMatchers",
			jewel: &crownjewelv1.CrownJewel{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &crownjewelv1.CrownJewelSpec{},
			},
			wantErr: require.Error,
		},
		{
			name: "MissingMetadata",
			jewel: &crownjewelv1.CrownJewel{
				Spec: &crownjewelv1.CrownJewelSpec{
					TeleportMatchers: []*crownjewelv1.TeleportMatcher{
						{
							Kinds: []string{"kind1"},
							Names: []string{"name1"},
							Labels: []*labelv1.Label{
								{
									Name:   "label1",
									Values: []string{"value1"},
								},
							},
						},
					},
				},
			},
			wantErr: require.Error,
		},
		{
			name: "EmptyName",
			jewel: &crownjewelv1.CrownJewel{
				Metadata: &headerv1.Metadata{
					Name: "",
				},
				Spec: &crownjewelv1.CrownJewelSpec{
					TeleportMatchers: []*crownjewelv1.TeleportMatcher{
						{
							Kinds: []string{"kind1"},
							Names: []string{"name1"},
							Labels: []*labelv1.Label{
								{
									Name:   "label1",
									Values: []string{"value1"},
								},
							},
						},
					},
					AwsMatchers: []*crownjewelv1.AWSMatcher{
						{
							Types: []string{"type1"},
							Arns:  []string{"arn1"},
							Tags: []*crownjewelv1.AWSTag{
								{
									Key:    "key1",
									Values: []*wrapperspb.StringValue{wrapperspb.String("value1")},
								},
							},
						},
					},
				},
			},
			wantErr: require.Error,
		},
		{
			name: "EmptyTeleportMatcherKinds",
			jewel: &crownjewelv1.CrownJewel{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &crownjewelv1.CrownJewelSpec{
					TeleportMatchers: []*crownjewelv1.TeleportMatcher{
						{
							Kinds: []string{},
							Names: []string{"name1"},
							Labels: []*labelv1.Label{
								{
									Name:   "label1",
									Values: []string{"value1"},
								},
							},
						},
					},
					AwsMatchers: []*crownjewelv1.AWSMatcher{
						{
							Types: []string{"type1"},
							Arns:  []string{"arn1"},
							Tags: []*crownjewelv1.AWSTag{
								{
									Key:    "key1",
									Values: []*wrapperspb.StringValue{wrapperspb.String("value1")},
								},
							},
						},
					},
				},
			},
			wantErr: require.Error,
		},
		{
			name: "EmptyAWSMatcherKinds",
			jewel: &crownjewelv1.CrownJewel{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &crownjewelv1.CrownJewelSpec{
					TeleportMatchers: []*crownjewelv1.TeleportMatcher{
						{
							Kinds: []string{"type2"},
							Names: []string{"name1"},
							Labels: []*labelv1.Label{
								{
									Name:   "label1",
									Values: []string{"value1"},
								},
							},
						},
					},
					AwsMatchers: []*crownjewelv1.AWSMatcher{
						{
							Types: []string{},
							Arns:  []string{"arn1"},
							Tags: []*crownjewelv1.AWSTag{
								{
									Key:    "key1",
									Values: []*wrapperspb.StringValue{wrapperspb.String("value1")},
								},
							},
						},
					},
				},
			},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := crownjewel.ValidateCrownJewel(tt.jewel)
			tt.wantErr(t, err)
		})
	}
}
