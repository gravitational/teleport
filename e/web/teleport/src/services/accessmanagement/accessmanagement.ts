import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import type {
  CreateAccessListRequest,
  AccessList,
  AccessListMember,
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
      roles: spec.grants?.roles || [],
    },
    audit: {
      frequency: spec.audit?.frequency || '',
    },
    ownershipRequires: {
      roles: spec.ownership_requires?.roles || [],
    },
    membershipRequires: {
      roles: spec.membership_requires?.roles || [],
    },
  };
}

function makeMembers(json: any): AccessListMember[] {
  if (!json) {
    return [];
  }
  return json.map(m => ({ ...m, addedBy: m.added_by }));
}
