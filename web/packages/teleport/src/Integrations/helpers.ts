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

import { IntegrationLike } from 'teleport/Integrations/IntegrationList';
import {
  getStatusCodeTitle,
  Integration,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { Status } from './types';

const HEALTHY = {
  status: Status.Healthy,
  label: 'Healthy',
  tooltip: 'Integration is connected and working.',
};

const DRAFT = {
  status: Status.Draft,
  label: 'Draft',
  tooltip: 'Integration setup has not been completed.',
};

const FAILED = (tooltip: string) => ({
  status: Status.Failed,
  label: 'Failed',
  tooltip,
});

export function getStatus(item: IntegrationLike): {
  status: Status;
  label: string;
  tooltip: string;
} {
  if (item.resourceType === 'integration') {
    return HEALTHY;
  }

  if (item.resourceType === 'external-audit-storage') {
    return item.statusCode === IntegrationStatusCode.Draft ? DRAFT : HEALTHY;
  }

  switch (item.statusCode) {
    case IntegrationStatusCode.Unknown:
      return {
        status: Status.Unknown,
        label: 'Healthy',
        tooltip: 'Integration is connected and working.',
      };
    case IntegrationStatusCode.Running:
      return HEALTHY;
    case IntegrationStatusCode.Draft:
      return DRAFT;
    case IntegrationStatusCode.SlackNotInChannel:
      return {
        status: Status.Issues,
        label: 'Issues',
        tooltip:
          'The Slack integration must be invited to the default channel in order to receive access request notifications.',
      };
    case IntegrationStatusCode.Unauthorized:
      return FAILED(
        'The integration was denied access. This could be a result of revoked authorization on the 3rd party provider. Try removing and re-connecting the integration.'
      );
    case IntegrationStatusCode.OktaConfigError:
      return FAILED(
        `There was an error with the integration's configuration.${item.status?.errorMessage ? ` ${item.status.errorMessage}` : ''}`
      );
    default:
      return FAILED('Integration failed due to an unknown error.');
  }
}

export function getStatusAndLabel(integration: Integration): {
  labelKind: LabelKind;
  status: string;
} {
  const { status: modifiedStatus } = getStatus(integration);
  const statusCode = integration.statusCode;
  const title = getStatusCodeTitle(statusCode);

  switch (modifiedStatus) {
    case Status.Healthy:
      return { labelKind: 'success', status: title };
    case Status.Failed:
      return { labelKind: 'danger', status: title };
    default:
      return { labelKind: 'secondary', status: title };
  }
}
