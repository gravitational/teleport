import { Role } from 'teleport/services/resources';

import {
  AccessList,
  AccessListMetadata,
  AccessListSpecRequest,
  MemberSpecRequest,
} from './types';

/**
 * Access lists created with a preset provides a web UI that guides admins on
 * defining access to resources. It basically provides a fancier role editor
 * where admin isn't necessarily aware that roles are involved.
 *
 * The preset values itself just determines what kind of actions Teleport
 * will perform on roles and how Teleport will assign these roles as owner/member
 * grants in the backend.
 *
 * Empty preset means a regular access list and has no special UI behaviors.
 *
 * These are backend expected values.
 */
export type AccessListPreset = 'long-term' | 'short-term' | '';

export type AccessListWithPresetRequest = {
  presetType: AccessListPreset;
  accessList: {
    spec: AccessListSpecRequest;
    metadata: AccessListMetadata;
    members: MemberSpecRequest[];
  };
  accessRoles: Role[];
};

export type UpdateAccessListWithPresetResponse = {
  accessList: AccessList;
  rolesToBeDeleted: string[];
};
