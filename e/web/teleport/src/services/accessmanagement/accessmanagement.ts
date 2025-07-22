import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';

import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import auth from 'teleport/services/auth/auth';
import { makeTraits } from 'teleport/services/user/makeUser';
import { withGenericUnsupportedError } from 'teleport/services/version/unsupported';

import {
  AccessList,
  AccessListCurrentUserAssignments,
  AccessListMember,
  AccessListOrigin,
  AccessListOwner,
  AccessListReview,
  AccessListType,
  AddMembersToAccessListRequest,
  IneligibleStatus,
  ReviewAccessListRequest,
  ReviewFrequency,
  ReviewFrequencyBackendParsableValue,
  UpsertAccessListRequest,
} from './types';

export const accessManagementService = {
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
              reviewers: spec.reviewers,
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
    const madeReq: UpsertAccessListRequest = {
      title: req.title || original.title,
      description: original.description, // cannot be edited.
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
        ? {
            roles: req.grants.roles,
            traits: req.grants.traits,
          }
        : original.grants,
      owner_grants: req.ownerGrants
        ? {
            roles: req.ownerGrants.roles,
            traits: req.ownerGrants.traits,
          }
        : original.ownerGrants,
      members: req.members
        ? req.members.map(m => ({
            name: m.name,
            joined: m.joined,
            expires: m.expires,
            reason: m.reason,
            added_by: m.addedBy,
            membership_kind: m.membershipKind,
          }))
        : original.members.map(m => ({
            name: m.name,
            joined: m.joined,
            expires: m.expires,
            reason: m.reason,
            added_by: m.addedBy,
            membership_kind: m.membershipKind,
          })),
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
};

export function makeAccessLists(json: any): AccessList[] {
  const accesslists = json || [];
  return accesslists.map(makeAccessList);
}

function originFromMetadataLabel(labels: object): AccessListOrigin {
  if (Object.keys(labels).includes('okta/org')) {
    return AccessListOrigin.Okta;
  }

  for (const [k, v] of Object.entries(labels)) {
    if (k === 'teleport.dev/origin' && v === 'aws-identity-center') {
      return AccessListOrigin.AwsIdentityCenter;
    }
  }

  return AccessListOrigin.Unspecified;
}

function makeAccessList(json: any): AccessList {
  const spec = json?.spec || { spec: {} };
  const metadata = json?.metadata || { metadata: {} };

  return {
    id: metadata?.name || '',
    origin: originFromMetadataLabel(metadata?.labels || {}),
    type: spec.type || AccessListType.Default,
    title: spec.title || '',
    description: spec.description || '',
    owners: makeOwners(spec.owners),
    members: makeMembers(json?.members),
    membersCount: json?.membersCount,
    memberListCount: json?.memberListCount,
    grants: {
      roles: spec.grants?.roles?.sort() || [],
      traits: makeTraits(spec.grants?.traits),
    },
    ownerGrants: {
      roles: spec.owner_grants?.roles?.sort() || [],
      traits: makeTraits(spec.owner_grants?.traits),
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
    },
    currentUserAssignments: makeAccessListCurrentUserAssignments(
      json?.current_user_assignments
    ),
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

function makeMembers(json: any): AccessListMember[] {
  if (!json) {
    return [];
  }
  return json.map(m => {
    return {
      name: m.name,
      reason: m.reason,
      addedBy: m.added_by,
      joined: new Date(m.joined),
      expires: new Date(m.expires),
      ineligibleReason: getIneligibleReason(m.ineligible_status),
      membershipKind: m.membership_kind,
    };
  });
}

function makeOwners(json: any): AccessListOwner[] {
  if (!json) {
    return [];
  }
  return json.map(o => {
    return {
      name: o.name,
      description: o.description,
      ineligibleReason: getIneligibleReason(o.ineligible_status),
      membershipKind: o.membership_kind,
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
