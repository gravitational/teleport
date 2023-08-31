import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import type {
  CreateAccessListRequest,
  AccessList,
  AccessListMember,
  UpdateAccessListRequest,
} from 'e-teleport/services/accessmanagement';

export const accessManagementService = {
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
  createAccessList(req: CreateAccessListRequest): Promise<AccessList> {
    return api
      .post(cfg.getAccessManagementListUrl(), req)
      .then(resp => makeAccessList(resp.accessList));
  },
  updateAccessList(a: Partial<AccessList>): Promise<void> {
    const req: UpdateAccessListRequest = {
      auditDuration: a.audit?.frequency,
      auditStartDate: a.audit?.nextDate,
      grants: {
        roles: a.grants?.roles,
        traits: a.grants?.traits,
      },
      members: a.members?.map(m => ({
        name: m.name,
        joined: m.joined,
        expires: m.expires,
        reason: m.reason,
        added_by: m.addedBy,
      })),
      owners: a.owners?.map(o => ({
        name: o.name,
        description: o.description,
      })),
      membership_requires: {
        roles: a.membershipRequires?.roles,
        traits: a.membershipRequires?.traits,
      },
      ownership_requires: {
        roles: a.ownershipRequires?.roles,
        traits: a.ownershipRequires?.traits,
      },
    };

    // TODO(lisa) backend wip
    console.log(req);
    return Promise.resolve();
  },
  deleteAccessList(accessListId): Promise<void> {
    // TODO(lisa) backend wip
    console.log(accessListId);
    return Promise.resolve();
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
    owners: spec.owners || [],
    members: makeMembers(spec.members),
    grants: {
      roles: spec.grants?.roles.sort() || [],
      traits: spec.grants?.grants || {},
    },
    audit: {
      frequency: spec.audit?.frequency || '',
      nextDate: spec.audit?.next_audit_date
        ? new Date(spec.audit?.next_audit_date)
        : undefined,
    },
    ownershipRequires: {
      roles: spec.ownership_requires?.roles.sort() || [],
      traits: spec.ownership_requires?.traits || {},
    },
    membershipRequires: {
      roles: spec.membership_requires?.roles.sort() || [],
      traits: spec.membership_requires?.traits || {},
    },
  };
}

function makeMembers(json: any): AccessListMember[] {
  if (!json) {
    return [];
  }
  return json.map(m => {
    return {
      ...m,
      addedBy: m.added_by,
      joined: new Date(m.joined),
      expires: new Date(m.expires),
    };
  });
}
