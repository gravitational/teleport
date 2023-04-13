/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export enum Stage {
  Initial,
  ClickIdentityProviders,
  IdentityProviders,
  ClickAddProvider,
  NewProvider,
  NewProviderFullScreen,
  SelectOpenIDConnect,
  OpenIDConnectSelected,
  ClickProviderURL,
  PastedProviderURL,
  ClickAudience,
  PastedAudience,
  ClickGetThumbprint,
  ThumbprintLoading,
  ThumbprintResult,
  SelectThumbprint,
  ThumbprintSelected,
  AddProvider,
  ProviderAdded,
  SelectProvider,
  ProviderView,
  ClickAssignRole,
  ShowAssignRoleModal,
  ClickCreateNewRole,
  CreateNewRole,
  SelectAudienceDropdown,
  ShowAudienceDropdown,
  ClickDiscoverAudience,
  DiscoverAudienceSelected,
  ClickNextPermissions,
  ConfigureRolePermissions,
  ClickCreatePolicy,
  CreatePolicy,
  ClickJSONTab,
  ShowJSONEditor,
  SelectJSONContents,
  JSONContentsSelected,
  PolicyJSONPasted,
  PolicyClickNextTags,
  PolicyTags,
  PolicyClickNextReview,
  PolicyReview,
  ClickPolicyName,
  PolicyHasName,
  ClickCreatePolicyButton,
  AssignPolicyToRole,
  ClickRefreshButton,
  PoliciesLoaded,
  ClickSearchBox,
  SearchForPolicy,
  SelectPolicy,
  PolicySelected,
  RoleClickNextTags,
  RoleTags,
  RoleClickNextReview,
  RoleReview,
  ClickRoleName,
  RoleHasName,
  ClickCreateRoleButton,
  ListRoles,
  ClickRole,
  ViewRole,
}

interface StageItem {
  kind: Stage;
  cursor: {
    top: number;
    left: number;
    click?: boolean;
  };
  duration?: number;
  end?: boolean;
}

export const STAGES: StageItem[] = [
  {
    kind: Stage.Initial,
    cursor: {
      top: 236,
      left: 200,
      click: false,
    },
    duration: 2000,
  },
  {
    kind: Stage.ClickIdentityProviders,
    cursor: {
      top: 401,
      left: 60,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.IdentityProviders,
    cursor: {
      top: 401,
      left: 200,
    },
    duration: 2000,
  },
  {
    kind: Stage.ClickAddProvider,
    cursor: {
      top: 221,
      left: 530,
      click: true,
    },
    duration: 1500,
  },
  {
    kind: Stage.NewProvider,
    cursor: {
      top: 221,
      left: 530,
    },
    end: true,
  },
  {
    kind: Stage.NewProviderFullScreen,
    cursor: {
      top: 221,
      left: 530,
    },
    duration: 1500,
  },
  {
    kind: Stage.SelectOpenIDConnect,
    cursor: {
      top: 281,
      left: 350,
      click: true,
    },
    duration: 2000,
  },
  {
    kind: Stage.OpenIDConnectSelected,
    cursor: {
      top: 281,
      left: 350,
    },
    duration: 2000,
  },
  {
    kind: Stage.ClickProviderURL,
    cursor: {
      top: 416,
      left: 160,
      click: true,
    },
    duration: 1500,
  },
  {
    kind: Stage.PastedProviderURL,
    cursor: {
      top: 416,
      left: 160,
    },
    duration: 1500,
  },
  {
    kind: Stage.ClickAudience,
    cursor: {
      top: 491,
      left: 160,
      click: true,
    },
    duration: 1500,
  },
  {
    kind: Stage.PastedAudience,
    cursor: {
      top: 491,
      left: 160,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickGetThumbprint,
    cursor: {
      top: 416,
      left: 400,
      click: true,
    },
    duration: 1000,
  },
  {
    kind: Stage.ThumbprintLoading,
    cursor: {
      top: 416,
      left: 400,
    },
    duration: 1000,
  },
  {
    kind: Stage.ThumbprintResult,
    cursor: {
      top: 416,
      left: 400,
    },
    duration: 1500,
  },
  {
    kind: Stage.SelectThumbprint,
    cursor: {
      top: 516,
      left: 100,
      click: true,
    },
    duration: 1500,
  },
  {
    kind: Stage.ThumbprintSelected,
    cursor: {
      top: 516,
      left: 100,
    },
    end: true,
  },
  {
    kind: Stage.AddProvider,
    cursor: {
      top: 666,
      left: 420,
      click: true,
    },
    duration: 2000,
  },
  {
    kind: Stage.ProviderAdded,
    cursor: {
      top: 516,
      left: 300,
    },
    duration: 2000,
  },
  {
    kind: Stage.SelectProvider,
    cursor: {
      top: 301,
      left: 70,
      click: true,
    },
    duration: 1500,
  },
  {
    kind: Stage.ProviderView,
    cursor: {
      top: 301,
      left: 70,
    },
    duration: 1500,
  },
  {
    kind: Stage.ClickAssignRole,
    cursor: {
      top: 221,
      left: 535,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.ShowAssignRoleModal,
    cursor: {
      top: 221,
      left: 535,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickCreateNewRole,
    cursor: {
      top: 446,
      left: 500,
      click: true,
    },
    end: true,
  },
  {
    kind: Stage.CreateNewRole,
    cursor: {
      top: 446,
      left: 500,
    },
    duration: 2000,
  },
  {
    kind: Stage.SelectAudienceDropdown,
    cursor: {
      top: 477,
      left: 300,
      click: true,
    },
    duration: 2000,
  },
  {
    kind: Stage.ShowAudienceDropdown,
    cursor: {
      top: 477,
      left: 300,
    },
    duration: 1000,
  },
  {
    kind: Stage.ClickDiscoverAudience,
    cursor: {
      top: 512,
      left: 300,
      click: true,
    },
    duration: 2000,
  },
  {
    kind: Stage.DiscoverAudienceSelected,
    cursor: {
      top: 512,
      left: 300,
    },
    duration: 1000,
  },
  {
    kind: Stage.ClickNextPermissions,
    cursor: {
      top: 722,
      left: 500,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.ConfigureRolePermissions,
    cursor: {
      top: 422,
      left: 500,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickCreatePolicy,
    cursor: {
      top: 256,
      left: 75,
      click: true,
    },
    end: true,
  },
  {
    kind: Stage.CreatePolicy,
    cursor: {
      top: 256,
      left: 75,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickJSONTab,
    cursor: {
      top: 210,
      left: 180,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.ShowJSONEditor,
    cursor: {
      top: 210,
      left: 180,
    },
    duration: 2500,
  },
  {
    kind: Stage.SelectJSONContents,
    cursor: {
      top: 320,
      left: 140,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.JSONContentsSelected,
    cursor: {
      top: 320,
      left: 140,
    },
    duration: 2500,
  },
  {
    kind: Stage.PolicyJSONPasted,
    cursor: {
      top: 320,
      left: 140,
    },
    duration: 2500,
  },
  {
    kind: Stage.PolicyClickNextTags,
    cursor: {
      top: 722,
      left: 530,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.PolicyTags,
    cursor: {
      top: 722,
      left: 530,
    },
    duration: 2500,
  },
  {
    kind: Stage.PolicyClickNextReview,
    cursor: {
      top: 722,
      left: 530,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.PolicyReview,
    cursor: {
      top: 722,
      left: 530,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickPolicyName,
    cursor: {
      top: 262,
      left: 230,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.PolicyHasName,
    cursor: {
      top: 262,
      left: 230,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickCreatePolicyButton,
    cursor: {
      top: 722,
      left: 530,
      click: true,
    },
    end: true,
  },
  {
    kind: Stage.AssignPolicyToRole,
    cursor: {
      top: 260,
      left: 550,
    },
    duration: 1000,
  },
  {
    kind: Stage.ClickRefreshButton,
    cursor: {
      top: 260,
      left: 550,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.PoliciesLoaded,
    cursor: {
      top: 260,
      left: 550,
    },
    duration: 1500,
  },
  {
    kind: Stage.ClickSearchBox,
    cursor: {
      top: 325,
      left: 120,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.SearchForPolicy,
    cursor: {
      top: 325,
      left: 120,
    },
    duration: 1500,
  },
  {
    kind: Stage.SelectPolicy,
    cursor: {
      top: 420,
      left: 36,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.PolicySelected,
    cursor: {
      top: 440,
      left: 50,
    },
    duration: 1500,
  },
  {
    kind: Stage.RoleClickNextTags,
    cursor: {
      top: 722,
      left: 530,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.RoleTags,
    cursor: {
      top: 722,
      left: 530,
    },
    duration: 2500,
  },
  {
    kind: Stage.RoleClickNextReview,
    cursor: {
      top: 722,
      left: 530,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.RoleReview,
    cursor: {
      top: 722,
      left: 530,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickRoleName,
    cursor: {
      top: 262,
      left: 230,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.RoleHasName,
    cursor: {
      top: 262,
      left: 230,
    },
    duration: 2500,
  },
  {
    kind: Stage.ClickCreateRoleButton,
    cursor: {
      top: 722,
      left: 530,
      click: true,
    },
    end: true,
  },
  {
    kind: Stage.ListRoles,
    cursor: {
      top: 722,
      left: 530,
    },
    duration: 2000,
  },
  {
    kind: Stage.ClickRole,
    cursor: {
      top: 142,
      left: 120,
      click: true,
    },
    duration: 2500,
  },
  {
    kind: Stage.ViewRole,
    cursor: {
      top: 182,
      left: 150,
    },
    end: true,
  },
];
