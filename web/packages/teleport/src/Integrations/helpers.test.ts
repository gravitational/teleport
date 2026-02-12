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

import { IntegrationLike } from 'teleport/Integrations/IntegrationList';
import { IntegrationStatusCode } from 'teleport/services/integrations';

import { getStatus } from './shared/StatusLabel';
import { Status } from './types';

test.each`
  type                        | code                                       | expected
  ${'integration'}            | ${IntegrationStatusCode.Draft}             | ${Status.Healthy}
  ${'integration'}            | ${IntegrationStatusCode.Running}           | ${Status.Healthy}
  ${'integration'}            | ${IntegrationStatusCode.Unauthorized}      | ${Status.Healthy}
  ${'integration'}            | ${IntegrationStatusCode.SlackNotInChannel} | ${Status.Healthy}
  ${'integration'}            | ${IntegrationStatusCode.Unknown}           | ${Status.Healthy}
  ${'integration'}            | ${IntegrationStatusCode.OtherError}        | ${Status.Healthy}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Draft}             | ${Status.Draft}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Running}           | ${Status.Healthy}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Unauthorized}      | ${Status.Healthy}
  ${'external-audit-storage'} | ${IntegrationStatusCode.SlackNotInChannel} | ${Status.Healthy}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Unknown}           | ${Status.Healthy}
  ${'external-audit-storage'} | ${IntegrationStatusCode.OtherError}        | ${Status.Healthy}
  ${'any'}                    | ${IntegrationStatusCode.Draft}             | ${Status.Draft}
  ${'any'}                    | ${IntegrationStatusCode.Running}           | ${Status.Healthy}
  ${'any'}                    | ${IntegrationStatusCode.Unauthorized}      | ${Status.Failed}
  ${'any'}                    | ${IntegrationStatusCode.SlackNotInChannel} | ${Status.Issues}
  ${'any'}                    | ${IntegrationStatusCode.Unknown}           | ${Status.Unknown}
  ${'any'}                    | ${IntegrationStatusCode.OtherError}        | ${Status.Failed}
`(
  'getStatus type $type with code $code returns $expected',
  async ({ type, code, expected }) => {
    const item: IntegrationLike = {
      name: '',
      kind: undefined,
      resourceType: type,
      statusCode: code,
    };
    const { status } = getStatus(item);
    expect(status).toBe(expected);
  }
);
