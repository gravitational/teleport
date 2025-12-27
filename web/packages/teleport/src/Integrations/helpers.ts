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

import { LabelKind } from 'design/Label';

import {
  getStatusCodeTitle,
  Integration,
  IntegrationKind,
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';

import { IntegrationLike, IntegrationTag, Status } from './types';

export function getStatus(item: IntegrationLike): Status {
  if (item.resourceType === 'integration') {
    return Status.Success;
  }

  if (item.resourceType === 'external-audit-storage') {
    switch (item.statusCode) {
      case IntegrationStatusCode.Draft:
        return Status.Warning;
      default:
        return Status.Success;
    }
  }

  switch (item.statusCode) {
    case IntegrationStatusCode.Unknown:
      return null;
    case IntegrationStatusCode.Running:
      return Status.Success;
    case IntegrationStatusCode.SlackNotInChannel:
      return Status.Warning;
    case IntegrationStatusCode.Draft:
      return Status.Warning;
    default:
      return Status.Error;
  }
}

export function getStatusAndLabel(integration: Integration): {
  labelKind: LabelKind;
  status: string;
} {
  const modifiedStatus = getStatus(integration);
  const statusCode = integration.statusCode;
  const title = getStatusCodeTitle(statusCode);

  switch (modifiedStatus) {
    case Status.Success:
      return { labelKind: 'success', status: title };
    case Status.Error:
      return { labelKind: 'danger', status: title };
    case Status.Warning:
      return { labelKind: 'warning', status: title };
    default:
      return { labelKind: 'secondary', status: title };
  }
}

export function integrationKindToTags(k: IntegrationKind): IntegrationTag[] {
  switch (k) {
    case IntegrationKind.AwsOidc:
    case IntegrationKind.AzureOidc:
      return ['idp'];

    case IntegrationKind.AwsRa:
    case IntegrationKind.ExternalAuditStorage:
    case IntegrationKind.GitHub:
      return ['resourceaccess'];

    default:
      return [];
  }
}

export function pluginKindToIntegrationTags(p: PluginKind): IntegrationTag[] {
  switch (p) {
    case 'slack':
    case 'opsgenie':
    case 'servicenow':
    case 'jira':
    case 'pagerduty':
    case 'email':
    case 'discord':
    case 'mattermost':
    case 'msteams':
    case 'datadog':
      return ['notifications'];

    case 'okta':
    case 'scim':
    case 'aws-identity-center':
      return ['idp', 'scim'];

    case 'jamf':
    case 'intune':
      return ['devicetrust'];

    case 'entra-id':
      return ['idp'];

    case 'openai':
      return [];

    default:
      return [];
  }
}

export function integrationLikeToIntegrationTags(
  i: IntegrationLike
): IntegrationTag[] {
  switch (i.resourceType) {
    case 'integration':
    case 'external-audit-storage':
      return integrationKindToTags(i.kind);
    case 'plugin':
      return pluginKindToIntegrationTags(i.kind);
  }
}

export function filterByIntegrationTags(
  l: IntegrationLike[],
  s: IntegrationTag[]
): IntegrationLike[] {
  return l.filter(i => {
    if (s.length) {
      if (!s.some(tags => integrationLikeToIntegrationTags(i).includes(tags))) {
        return false;
      }
    }

    return true;
  });
}
