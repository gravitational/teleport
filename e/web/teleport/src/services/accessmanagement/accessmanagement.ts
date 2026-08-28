import { SortType } from 'design/DataTable/types';
import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';

import cfg from 'e-teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import { MfaChallengeResponse } from 'teleport/services/mfa';
import { makeTraits } from 'teleport/services/user/makeUser';
import {
  isPathNotFoundError,
  withGenericUnsupportedError,
} from 'teleport/services/version/unsupported';

import {
  AccessListPreset,
  AccessListWithPresetRequest,
  GenerateTerraformConfigRequest,
  UpdateAccessListWithPresetResponse,
} from './preset';
import {
  AccessList,
  AccessListCurrentUserAssignments,
  AccessListMember,
  AccessListOrigin,
  AccessListOwner,
  AccessListReview,
  AccessListType,
  AccessListUserAssignments,
  AddMembersToAccessListRequest,
  IneligibleStatus,
  ReviewAccessListRequest,
  ReviewFrequency,
  ReviewFrequencyBackendParsableValue,
  ScopedRoleListItem,
  ScopedRoleGrant,
  UpsertAccessListRequest,
  makeAccessListGrantRequest,
} from './types';

export const accessManagementService = {
  fetchUserAccessLists(
    {
      username,
      pageSize,
      pageToken,
    }: { username: string; pageSize?: number; pageToken?: string },
    abortSignal?: AbortSignal
  ): Promise<{
    accessLists: AccessList[];
    nextKey: string;
    totalCount: number;
  }> {
    const urlParams = new URLSearchParams();
    if (pageSize) {
      urlParams.set('limit', pageSize.toString());
    }
    if (pageToken) {
      urlParams.set('startKey', pageToken);
    }

    const url = urlParams.size
      ? `${cfg.getUserAccessListsUrl(username)}?${urlParams}`
      : cfg.getUserAccessListsUrl(username);

    return api.get(url, abortSignal).then(resp => ({
      accessLists: makeAccessLists(resp.accessLists),
      nextKey: resp.startKey || '',
      totalCount: resp.totalCount || 0,
    }));
  },
  fetchAccessListSuggestions(accessRequestId: string): Promise<AccessList[]> {
    return api
      .get(cfg.getAccessListSuggestionsUrl(accessRequestId))
      .then(resp => makeAccessLists(resp.accessLists));
  },
  fetchAccessLists(abortSignal?: AbortSignal): Promise<AccessList[]> {
    return api
      .get(cfg.getAccessManagementListUrl(), abortSignal)
      .then(resp => makeAccessLists(resp.accessLists));
  },
  fetchAccessListsV2(
    params: {
      search?: string;
      limit?: number;
      sort?: SortType;
      startKey?: string;
      owners?: string[];
      origin?: string;
    },
    abortSignal?: AbortSignal
  ): Promise<ResourcesResponse<AccessList>> {
    return api
      .get(cfg.getAccessManagementListUrlV2(params), abortSignal)
      .then(resp => {
        return {
          agents: makeAccessLists(resp.accessLists),
          startKey: resp.startKey,
        };
      })
      .catch(err => {
        // TODO (avatus): DELETE in v21
        if (isPathNotFoundError(err)) {
          const resp = this.fetchAccessLists(abortSignal);
          return {
            agents: makeAccessLists(resp),
            startKey: '',
          };
        } else {
          throw err;
        }
      });
  },
  fetchReviews(
    accessListId: string,
    params: { startKey: string; limit: number },
    abortSignal?: AbortSignal
  ): Promise<{ reviews: AccessListReview[]; startKey: string }> {
    return (
      api
        .get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: {
              accessListId,
              ...params,
            },
          }),
          abortSignal
        )
        .then(resp => {
          const madeReviews = resp.reviews?.map(r => {
            const spec = r.spec ?? {};
            return {
              notes: spec.notes,
              reviewDate: new Date(spec.review_date),
              reviewers:
                r.reviewersInfo?.map(info => ({
                  name: info.username,
                  ...(info.display && {
                    displayPrimary: info.display.primary,
                    displaySecondary: info.display.secondary,
                  }),
                })) || (spec.reviewers || []).map(name => ({ name })),
              raw: r,
            };
          });
          return {
            reviews: madeReviews ?? [],
            startKey: resp.startKey,
          };
        })
        // TODO(kimlisa): DELETE IN v20.0
        .catch(err => withGenericUnsupportedError(err, '18.1.0'))
    );
  },
  fetchAccessList(accessListId: string): Promise<AccessList> {
    return api
      .get(cfg.getAccessManagementListUrl(accessListId))
      .then(resp => makeAccessList(resp.accessList));
  },
  addMembersToAccessList(
    accessListId: string,
    req: AddMembersToAccessListRequest
  ): Promise<void> {
    return api.post(cfg.getAccessListMembersUrl(accessListId), req);
  },
  createAccessList(req: UpsertAccessListRequest): Promise<AccessList> {
    return api
      .post(cfg.getAccessManagementListUrl(), req)
      .then(resp => makeAccessList(resp.accessList));
  },
  async createAccessListWithPreset(
    req: AccessListWithPresetRequest,
    mfaResponse?: MfaChallengeResponse
  ): Promise<AccessList> {
    if (!mfaResponse) {
      // Reusable token is required b/c endpoint makes
      // 2x grpc calls that both require re-authn.
      const challenge = await auth.getMfaChallenge({
        scope: MfaChallengeScope.ADMIN_ACTION,
        allowReuse: true,
        isMfaRequiredRequest: {
          admin_action: {},
        },
      });
      mfaResponse = await auth.getMfaChallengeResponse(challenge);
    }

    return api
      .post(
        cfg.getAccessListWithPresetUrl({ action: 'create' }),
        req,
        undefined,
        mfaResponse
      )
      .then(resp => makeAccessList(resp.accessList));
  },
  async updateAccessListWithPreset(
    req: AccessListWithPresetRequest
  ): Promise<UpdateAccessListWithPresetResponse> {
    // Reusable token is required b/c endpoint makes
    // 2x grpc calls that both require re-authn.
    const challenge = await auth.getMfaChallenge({
      scope: MfaChallengeScope.ADMIN_ACTION,
      allowReuse: true,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    });

    const challengeResponse = await auth.getMfaChallengeResponse(challenge);

    return api
      .put(
        cfg.getAccessListWithPresetUrl({
          action: 'update',
          accessListId: req.accessList.metadata.name,
        }),
        req,
        undefined,
        challengeResponse
      )
      .then(resp => ({
        accessList: makeAccessList(resp.accessList),
        rolesToBeDeleted: resp.rolesToBeDeleted ?? [],
      }));
  },
  async deleteAccessListWithPreset(
    accessListId: string,
    mfaResponse?: MfaChallengeResponse
  ): Promise<string[]> {
    if (!mfaResponse) {
      mfaResponse = await auth.getMfaChallengeResponseForAdminAction(true);
    }
    return api
      .delete(
        cfg.getAccessListWithPresetUrl({ action: 'delete', accessListId }),
        null,
        mfaResponse
      )
      .then(resp => resp?.roles ?? []);
  },
  reviewAccessList(req: ReviewAccessListRequest): Promise<Date> {
    const madeReq = {
      access_list: req.name,
      reviewers: [req.reviewer],
      review_date: new Date(),
      notes: req.notes,
      changes: {
        membership_requirements_changed: req.membershipRequires,
        removed_members: req.membersDeleted?.map(m => m.name),
        review_frequency_changed:
          convertReviewFrequencyIntoBackendParsableValue(
            req.auditRecurrence.frequency
          ),
        review_day_of_month_changed: req.auditRecurrence.dayOfMonth,
      },
    };
    return api
      .post(cfg.getAccessListReviewUrl(req.name), madeReq)
      .then(req => new Date(req.nextAuditDate));
  },
  // updateAccessList will construct a backend model of AccessList
  // with the original object, replacing field values requested to
  // be updated.
  updateAccessList({
    req,
    original,
  }: {
    req: Partial<AccessList>;
    original: AccessList;
  }): Promise<AccessList> {
    const madeReq = makeAccessListForUpdate({ req, original });
    return api
      .put(cfg.getAccessManagementListUrl(original.id), madeReq)
      .then(resp => makeAccessList(resp.accessList));
  },
  async deleteAccessList(accessListId: string): Promise<void> {
    const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(true);
    return api.delete(
      cfg.getAccessManagementListUrl(accessListId),
      null,
      mfaResponse
    );
  },
  async generateTerraformConfig(
    req: GenerateTerraformConfigRequest,
    signal?: AbortSignal
  ) {
    const resp = await api.post(
      cfg.getAccessListWithPresetUrl({
        action: 'terraform',
      }),
      req,
      signal
    );

    return resp as {
      terraform: string;
    };
  },
  async fetchRootScopedRoles(
    params: fetchRootScopedRolesParams,
    abortSignal?: AbortSignal
  ): Promise<{ roles: ScopedRoleListItem[]; startKey: string }> {
    const resp = await api.get(cfg.getRootScopedRolesUrl(params), abortSignal);
    return {
      roles: makeScopedRoleListItems(resp.roles),
      startKey: resp.startKey || '',
    };
  },
};

export type fetchRootScopedRolesParams = {
  limit?: number;
  startKey?: string;
  filter?: string;
};

type UpdateRequest = {
  req: Partial<AccessList>;
  original: AccessList;
};

export function makeAccessListForUpdate({
  req,
  original,
  withoutMembers = false,
}: UpdateRequest & {
  withoutMembers?: boolean;
}): UpsertAccessListRequest {
  return {
    type: req.type || original.type,
    title: req.title || original.title,
    description:
      req.description !== undefined ? req.description : original.description,
    audit: req.audit
      ? {
          next_audit_date: req.audit.nextDate,
          recurrence: {
            frequency: convertReviewFrequencyIntoBackendParsableValue(
              req.audit.recurrence.frequency
            ),
            day_of_month: req.audit.recurrence.dayOfMonth,
          },
        }
      : {
          next_audit_date: original.audit.nextDate,
          recurrence: {
            frequency: convertReviewFrequencyIntoBackendParsableValue(
              original.audit.recurrence.frequency
            ),
            day_of_month: original.audit.recurrence.dayOfMonth,
          },
        },
    grants: req.grants
      ? makeAccessListGrantRequest(req.grants)
      : makeAccessListGrantRequest(original.grants),
    owner_grants: req.ownerGrants
      ? makeAccessListGrantRequest(req.ownerGrants)
      : makeAccessListGrantRequest(original.ownerGrants),
    ...(!withoutMembers && {
      members: makeAccessListMembersForUpdate({ req, original }),
    }),
    owners: req.owners
      ? req.owners.map(o => ({
          name: o.name,
          description: o.description,
          membership_kind: o.membershipKind,
        }))
      : original.owners.map(o => ({
          name: o.name,
          description: o.description,
          membership_kind: o.membershipKind,
        })),
    membership_requires: req.membershipRequires
      ? {
          roles: req.membershipRequires.roles,
          traits: req.membershipRequires.traits,
        }
      : original.membershipRequires,
    ownership_requires: req.ownershipRequires
      ? {
          roles: req.ownershipRequires.roles,
          traits: req.ownershipRequires.traits,
        }
      : original.ownershipRequires,
  };
}

export function makeAccessListMembersForUpdate({
  req,
  original,
}: UpdateRequest) {
  return req.members
    ? req.members.map(m => ({
        name: m.name,
        title: m.title,
        joined: m.joined,
        expires: m.expires,
        reason: m.reason,
        added_by: m.addedBy,
        membership_kind: m.membershipKind,
      }))
    : original.members.map(m => ({
        name: m.name,
        joined: m.joined,
        title: m.title,
        expires: m.expires,
        reason: m.reason,
        added_by: m.addedBy,
        membership_kind: m.membershipKind,
      }));
}

function makeAccessLists(json: any): AccessList[] {
  const accesslists = json || [];
  return accesslists.map(makeAccessList);
}

function originFromMetadataLabel(
  labels: object,
  type: AccessListType
): AccessListOrigin {
  if (Object.keys(labels).includes('okta/org')) {
    return AccessListOrigin.Okta;
  }

  for (const [k, v] of Object.entries(labels)) {
    if (k === 'teleport.dev/origin' && v === 'aws-identity-center') {
      return AccessListOrigin.AwsIdentityCenter;
    }
  }

  for (const [k, v] of Object.entries(labels)) {
    if (k === 'teleport.dev/origin' && v === 'entra-id') {
      return AccessListOrigin.EntraID;
    }
  }

  // SCIM-synced lists are identified by spec.type rather than an origin label,
  // but should be displayed as another synced source in the UI.
  if (type === AccessListType.Scim) {
    return AccessListOrigin.Scim;
  }

  return AccessListOrigin.Unspecified;
}

function getPresetTypeFromMetadataLabel(labels: object): AccessListPreset {
  if (!labels) {
    return '';
  }
  return labels['teleport.internal/access-list-preset'] ?? '';
}

export function getPresetRolesFromMetadataLabel(labels: object): string[] {
  if (!labels) {
    return [];
  }
  const roles = labels['teleport.internal/access-list-preset-roles'];
  return roles ? roles.split(',') : [];
}

export function makeAccessList(json: any): AccessList {
  const spec = json?.spec || { spec: {} };
  const metadata = json?.metadata || {};
  const type = spec.type || AccessListType.Default;
  const userDisplays = json?.user_displays || {};

  return {
    id: metadata?.name || '',
    metadata,
    origin: originFromMetadataLabel(metadata?.labels || {}, type),
    preset: getPresetTypeFromMetadataLabel(metadata?.labels || {}),
    type,
    title: spec.title || '',
    description: spec.description || '',
    owners: makeOwners(spec.owners, userDisplays),
    members: makeMembers(json?.members, userDisplays),
    membersCount: json?.membersCount,
    memberListCount: json?.memberListCount,
    grants: {
      roles: spec.grants?.roles?.sort() || [],
      traits: makeTraits(spec.grants?.traits),
      scopedRoles: makeScopedRoleGrants(spec.grants?.scoped_roles),
    },
    ownerGrants: {
      roles: spec.owner_grants?.roles?.sort() || [],
      traits: makeTraits(spec.owner_grants?.traits),
      scopedRoles: makeScopedRoleGrants(spec.owner_grants?.scoped_roles),
    },
    audit: {
      recurrence: {
        frequency: spec.audit?.recurrence?.frequency || '',
        dayOfMonth: spec.audit?.recurrence?.day_of_month || '',
      },
      nextDate: spec.audit?.next_audit_date
        ? new Date(spec.audit?.next_audit_date)
        : undefined,
    },
    ownershipRequires: {
      roles: spec.ownership_requires?.roles?.sort() || [],
      traits: spec.ownership_requires?.traits || {},
    },
    membershipRequires: {
      roles: spec.membership_requires?.roles?.sort() || [],
      traits: spec.membership_requires?.traits || {},
    },
    inheritedMemberGrants: {
      roles: json?.inherited_member_grants?.roles || [],
      traits: makeTraits(json?.inherited_member_grants?.traits),
      scopedRoles: makeScopedRoleGrants(
        json?.inherited_member_grants?.scoped_roles
      ),
    },
    currentUserAssignments: makeAccessListCurrentUserAssignments(
      json?.current_user_assignments
    ),
    userAssignments: makeAccessListUserAssignments(json?.user_assignments),
  };
}

const makeAccessListCurrentUserAssignments = (
  json: any
): AccessListCurrentUserAssignments => {
  if (!json) {
    return undefined;
  }

  const { ownership_type, membership_type } = json as {
      ownership_type: number;
      membership_type: number;
    },
    ownershipType = intToUserAssignmentType(ownership_type),
    membershipType = intToUserAssignmentType(membership_type);

  return { ownershipType, membershipType };
};

const makeAccessListUserAssignments = (
  json: any
): AccessListUserAssignments => {
  if (!json) {
    return undefined;
  }

  const { ownership_type, membership_type } = json as {
      ownership_type: number;
      membership_type: number;
    },
    ownershipType = intToUserAssignmentType(ownership_type),
    membershipType = intToUserAssignmentType(membership_type);

  return { ownershipType, membershipType };
};

const intToUserAssignmentType = (
  type: number
): AccessListUserAssignmentType => {
  switch (type) {
    case 1:
      return AccessListUserAssignmentType.EXPLICIT;
    case 2:
      return AccessListUserAssignmentType.INHERITED;
    case 0:
    default:
      return AccessListUserAssignmentType.UNSPECIFIED;
  }
};

export function getIneligibleReason(ineligibleStatus: IneligibleStatus) {
  if (ineligibleStatus === IneligibleStatus.UserNotExist) {
    return 'User does not exist';
  }
  if (ineligibleStatus === IneligibleStatus.MissingRequirements) {
    return 'User does not meet the roles or traits required';
  }
  if (ineligibleStatus === IneligibleStatus.Expired) {
    return `User's membership has expired`;
  }
  return '';
}

type UserDisplays = Record<string, { primary?: string; secondary?: string }>;

function makeMembers(
  json: any,
  userDisplays: UserDisplays
): AccessListMember[] {
  if (!json) {
    return [];
  }
  return json.map(m => {
    const display = userDisplays[m.name];
    const addedByDisplay = userDisplays[m.added_by];
    return {
      name: m.name,
      title: m.title,
      reason: m.reason,
      addedBy: m.added_by,
      joined: new Date(m.joined),
      expires: new Date(m.expires),
      ineligibleReason: getIneligibleReason(m.ineligible_status),
      membershipKind: m.membership_kind,
      ...(display && {
        displayPrimary: display.primary,
        displaySecondary: display.secondary,
      }),
      ...(addedByDisplay && {
        addedByDisplayPrimary: addedByDisplay.primary,
        addedByDisplaySecondary: addedByDisplay.secondary,
      }),
    };
  });
}

function makeOwners(json: any, userDisplays: UserDisplays): AccessListOwner[] {
  if (!json) {
    return [];
  }
  return json.map(o => {
    const display = userDisplays[o.name];
    return {
      title: o.title,
      name: o.name,
      description: o.description,
      ineligibleReason: getIneligibleReason(o.ineligible_status),
      membershipKind: o.membership_kind,
      ...(display && {
        displayPrimary: display.primary,
        displaySecondary: display.secondary,
      }),
    };
  });
}

export function convertReviewFrequencyIntoBackendParsableValue(
  frequency: ReviewFrequency
): ReviewFrequencyBackendParsableValue {
  switch (frequency) {
    case ReviewFrequency.OneMonth:
      return '1m';
    case ReviewFrequency.ThreeMonths:
      return '3m';
    case ReviewFrequency.SixMonths:
      return '6m';
    case ReviewFrequency.OneYear:
      return '12m';
    default:
      return null;
  }
}

const makeScopedRoleGrants = (json: any): ScopedRoleGrant[] => {
  if (!json) {
    return [];
  }

  return json.map((grant: any): ScopedRoleGrant => {
    return {
      role: grant.role,
      scope: grant.scope,
    };
  });
};

const makeScopedRoleListItems = (json: any): ScopedRoleListItem[] => {
  if (!json) {
    return [];
  }

  return json.map((role: any): ScopedRoleListItem => {
    return {
      name: role.name,
      scope: role.scope,
      assignableScopes: role.assignableScopes ?? [],
    };
  });
};
