import api from 'teleport/services/api';
import { makeTraits } from 'teleport/services/user/makeUser';

import cfg from 'e-teleport/config';

import {
  UpsertAccessListRequest,
  AccessList,
  AccessListMember,
  AccessListOwner,
  IneligibleStatus,
  AddMembersToAccessListRequest,
} from 'e-teleport/services/accessmanagement';

export const accessManagementService = {
  fetchAccessListSuggestions(accessRequestId: string): Promise<AccessList[]> {
    return api
      .get(cfg.getAccessListSuggestionsUrl(accessRequestId))
      .then(resp => makeAccessLists(resp.accessLists));
  },
  fetchAccessLists(): Promise<AccessList[]> {
    return api
      .get(cfg.getAccessManagementListUrl())
      .then(resp => makeAccessLists(resp.accessLists));
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
  // updateAccessList will construct a backend model of AccessList
  // with the original object, replacing field values requested to
  // be updated.
  updateAccessList({
    req,
    original,
  }: {
    req: Partial<AccessList>;
    original: AccessList;
  }): Promise<void> {
    const madeReq: UpsertAccessListRequest = {
      title: original.title, // cannot be edited.
      description: original.description, // cannot be edited.
      audit: req.audit
        ? {
            next_audit_date: req.audit.nextDate,
            frequency: req.audit.frequency,
          }
        : {
            next_audit_date: original.audit.nextDate,
            frequency: original.audit.frequency,
          },
      grants: req.grants
        ? {
            roles: req.grants.roles,
            traits: req.grants.traits,
          }
        : original.grants,
      members: req.members
        ? req.members.map(m => ({
            name: m.name,
            joined: m.joined,
            expires: m.expires,
            reason: m.reason,
            added_by: m.addedBy,
          }))
        : original.members.map(m => ({
            name: m.name,
            joined: m.joined,
            expires: m.expires,
            reason: m.reason,
            added_by: m.addedBy,
          })),
      owners: req.owners
        ? req.owners.map(o => ({
            name: o.name,
            description: o.description,
          }))
        : original.owners.map(o => ({
            name: o.name,
            description: o.description,
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

    return api.put(cfg.getAccessManagementListUrl(original.id), madeReq);
  },
  deleteAccessList(accessListId: string): Promise<void> {
    return api.delete(cfg.getAccessManagementListUrl(accessListId));
  },
};

export function makeAccessLists(json: any): AccessList[] {
  const accesslists = json || [];
  return accesslists.map(makeAccessList);
}

function makeAccessList(json: any): AccessList {
  const spec = json?.spec || { spec: {} };
  const metadata = json?.metadata || { metadata: {} };

  return {
    id: metadata?.name || '',
    title: spec.title || '',
    description: spec.description || '',
    owners: makeOwners(spec.owners),
    members: makeMembers(json?.members),
    membersCount: json?.membersCount,
    grants: {
      roles: spec.grants?.roles?.sort() || [],
      traits: makeTraits(spec.grants?.traits),
    },
    audit: {
      frequency: spec.audit?.frequency || '',
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
  };
}

function getIneligibleReason(ineligibleStatus: IneligibleStatus) {
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
    };
  });
}
