/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { ResourceKind } from '../Shared';
import { SelectResourceSpec } from './resources';

export const makeResourceSpec = (
  overrides: Partial<SelectResourceSpec> = {}
): SelectResourceSpec => {
  return Object.assign(
    {
      id: '',
      name: '',
      kind: ResourceKind.Application,
      icon: '',
      event: null,
      keywords: [],
      hasAccess: true,
    },
    overrides
  );
};

export const t_Application_NoAccess = makeResourceSpec({
  name: 'tango',
  kind: ResourceKind.Application,
  hasAccess: false,
});
export const u_Database_NoAccess = makeResourceSpec({
  name: 'uniform',
  kind: ResourceKind.Database,
  hasAccess: false,
});
export const v_Desktop_NoAccess = makeResourceSpec({
  name: 'victor',
  kind: ResourceKind.Desktop,
  hasAccess: false,
});
export const w_Kubernetes_NoAccess = makeResourceSpec({
  name: 'whiskey',
  kind: ResourceKind.Kubernetes,
  hasAccess: false,
});
export const x_Server_NoAccess = makeResourceSpec({
  name: 'xray',
  kind: ResourceKind.Server,
  hasAccess: false,
});
export const y_Saml_NoAccess = makeResourceSpec({
  name: 'yankee',
  kind: ResourceKind.SamlApplication,
  hasAccess: false,
});
export const z_Discovery_NoAccess = makeResourceSpec({
  name: 'zulu',
  kind: ResourceKind.Discovery,
  hasAccess: false,
});

export const NoAccessList: SelectResourceSpec[] = [
  t_Application_NoAccess,
  u_Database_NoAccess,
  v_Desktop_NoAccess,
  w_Kubernetes_NoAccess,
  x_Server_NoAccess,
  y_Saml_NoAccess,
  z_Discovery_NoAccess,
];

export const c_ApplicationGcp = makeResourceSpec({
  name: 'charlie',
  kind: ResourceKind.Application,
  keywords: ['gcp'],
});
export const a_DatabaseAws = makeResourceSpec({
  name: 'alpha',
  kind: ResourceKind.Database,
  keywords: ['aws'],
});
export const l_DesktopAzure = makeResourceSpec({
  name: 'linux',
  kind: ResourceKind.Desktop,
  keywords: ['azure'],
});
export const e_KubernetesSelfHosted_unguided = makeResourceSpec({
  name: 'echo',
  kind: ResourceKind.Kubernetes,
  unguidedLink: 'test.com',
  keywords: ['self-hosted'],
});
export const f_Server = makeResourceSpec({
  name: 'foxtrot',
  kind: ResourceKind.Server,
});
export const d_Saml = makeResourceSpec({
  name: 'delta',
  kind: ResourceKind.SamlApplication,
});
export const g_Application = makeResourceSpec({
  name: 'golf',
  kind: ResourceKind.Application,
});
export const k_Database = makeResourceSpec({
  name: 'kilo',
  kind: ResourceKind.Database,
});
export const i_Desktop = makeResourceSpec({
  name: 'india',
  kind: ResourceKind.Desktop,
});
export const j_Kubernetes = makeResourceSpec({
  name: 'juliette',
  kind: ResourceKind.Kubernetes,
});
export const h_Server = makeResourceSpec({
  name: 'hotel',
  kind: ResourceKind.Server,
});
export const l_Saml = makeResourceSpec({
  name: 'lima',
  kind: ResourceKind.SamlApplication,
});

export const kindBasedList: SelectResourceSpec[] = [
  c_ApplicationGcp,
  a_DatabaseAws,
  t_Application_NoAccess,
  l_DesktopAzure,
  e_KubernetesSelfHosted_unguided,
  u_Database_NoAccess,
  f_Server,
  w_Kubernetes_NoAccess,
  d_Saml,
  v_Desktop_NoAccess,
  g_Application,
  x_Server_NoAccess,
  k_Database,
  i_Desktop,
  z_Discovery_NoAccess,
  j_Kubernetes,
  h_Server,
  y_Saml_NoAccess,
  l_Saml,
];
