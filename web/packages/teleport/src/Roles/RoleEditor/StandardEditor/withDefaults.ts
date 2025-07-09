/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Role, RoleOptions } from 'teleport/services/resources';

import { defaultRoleVersion } from './standardmodel';

export type DeepPartial<T> = {
  [k in keyof T]?: T[k] extends object ? DeepPartial<T[k]> : T[k];
};

/**
 * Returns a "completed" model, emulating what `RoleV6.CheckAndSetDefaults`
 * does on the server side. These two functions must be kept in sync. Don't add
 * arbitrary defaults here unless they are returned the same way from the
 * server side, or the role editor will silently modify these fields on every
 * role it opens and saves without user intervention.
 */
export const withDefaults = (role: DeepPartial<Role>): Role => ({
  kind: 'role',
  version: defaultRoleVersion,

  ...role,

  metadata: {
    name: '',
    ...role.metadata,
  },

  spec: {
    ...role.spec,

    allow: {
      ...role.spec?.allow,
    },

    deny: {
      ...role.spec?.deny,
    },

    options: optionsWithDefaults(role.spec?.options),
  },
});

export const optionsWithDefaults = (
  options?: DeepPartial<RoleOptions>
): RoleOptions => {
  const defaults = defaultOptions();
  return {
    ...defaults,
    ...options,

    idp: {
      ...defaults.idp,
      ...options?.idp,

      saml: {
        ...defaults.idp.saml,
        ...options?.idp?.saml,
      },
    },

    record_session: {
      ...defaults.record_session,
      ...options?.record_session,
    },
  };
};

/**
 * Default options, exactly as returned by the server side for the empty
 * options object. This invariant must be held at all times, since this object
 * is used to resolve partial responses. Don't add arbitrary defaults here
 * unless they are returned the same way from the server side, or the role
 * editor will silently modify these options on every role it opens and saves
 * without user intervention.
 */
export const defaultOptions = (): RoleOptions => ({
  cert_format: 'standard',
  create_db_user: false,
  create_desktop_user: false,
  desktop_clipboard: true,
  desktop_directory_sharing: true,
  enhanced_recording: ['command', 'network'],
  forward_agent: false,
  idp: {
    saml: {
      enabled: true,
    },
  },
  max_session_ttl: '30h0m0s',
  pin_source_ip: false,
  record_session: {
    default: 'best_effort',
    desktop: true,
  },
  ssh_file_copy: true,
});
