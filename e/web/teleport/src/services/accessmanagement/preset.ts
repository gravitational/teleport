import { RoleSpec } from 'teleport/services/resources';

import { UpsertAccessListRequest } from './types';

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

/**
 * Expected backend values when requesting to create
 * an access list with a preset.
 */
export enum PresetType {
  Unspecified = 0,
  LongTerm = 1,
  ShortTerm = 2,
}

/**
 * Describes the role that Teleport will create
 * for an access list.
 */
export type AccessRole = {
  name_prefix: string;
  spec: RoleSpec;
};

export type UpsertAccessListWithPreset = UpsertAccessListRequest & {
  accessListId: string;
  presetType: PresetType;
  accessRoles: AccessRole[];
};
