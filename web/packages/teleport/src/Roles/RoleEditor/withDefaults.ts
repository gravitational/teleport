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

import { Role } from 'teleport/services/resources';

export type DeepPartial<T> = {
  [k in keyof T]?: T[k] extends object ? DeepPartial<T[k]> : T[k];
};

/**
 * Returns a "completed" model, emulating what `RoleV6.CheckAndSetDefaults`
 * does on the server side. These two functions must be kept in sync.
 */
export const withDefaults = (role: DeepPartial<Role>): Role => ({
  kind: 'role',
  version: '',

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

    options: {
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
      port_forwarding: true,
      record_session: {
        default: 'best_effort',
        desktop: true,
      },
      ssh_file_copy: true,

      ...role.spec?.options,
    },
  },
});
