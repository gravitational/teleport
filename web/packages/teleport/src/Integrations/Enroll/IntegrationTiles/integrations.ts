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

import { ResourceIconName } from 'design/ResourceIcon';

import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';

export type IntegrationTileSpec = {
  /**
   * In enterprise, resource type 'plugin' and type 'integration' are mixed.
   * This 'type' field is used to differentiate between this 'integration' type
   * and the plugin types defined for PluginBase found in:
   * https://github.com/gravitational/teleport.e/blob/62a53a71708366f1c314a8b32d99e09bc5c9b894/web/teleport/src/services/plugins/types.ts#L36
   */
  type: 'integration';
  kind: IntegrationKind;
  icon: ResourceIconName;
  name: string;
};

// Add new integrations here sorted by 'name' field.
const integrations: IntegrationTileSpec[] = [
  {
    type: 'integration',
    kind: IntegrationKind.ExternalAuditStorage,
    icon: 'aws',
    name: 'AWS External Audit Storage',
  },
  {
    type: 'integration',
    kind: IntegrationKind.AwsOidc,
    icon: 'aws',
    name: 'AWS OIDC Identity Provider',
  },
];

export function installableIntegrations() {
  const isOnpremEnterprise = cfg.isEnterprise && !cfg.isCloud;

  return integrations.filter(i => {
    // We only render external audit storage for OSS (with CTA buttons) or cloud edition.
    if (i.kind === IntegrationKind.ExternalAuditStorage && isOnpremEnterprise) {
      return false;
    }
    return true;
  });
}
