/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { Link as InternalLink } from 'react-router-dom';

import { ButtonIcon, Flex, Label, Text } from 'design';
import { ArrowLeft } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';
import { getStatusAndLabel } from 'teleport/Integrations/helpers';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import { IntegrationAwsOidc } from 'teleport/services/integrations';

export function AwsOidcTitle({
  integration,
  resource,
  tasks,
}: {
  integration: IntegrationAwsOidc;
  resource?: AwsResource;
  tasks?: boolean;
}) {
  const { status, labelKind } = getStatusAndLabel(integration);
  const content = getContent(integration, resource, tasks);

  return (
    <Flex alignItems="center" data-testid="aws-oidc-title">
      <HoverTooltip position="bottom" tipContent={content.helper}>
        <ButtonIcon as={InternalLink} to={content.to} aria-label="back">
          <ArrowLeft size="medium" />
        </ButtonIcon>
      </HoverTooltip>
      <Text bold fontSize={6} mx={2}>
        {content.content}
      </Text>
      <Label kind={labelKind} aria-label="status" px={3} ml={3}>
        {status}
      </Label>
    </Flex>
  );
}

function getContent(
  integration: IntegrationAwsOidc,
  resource?: AwsResource,
  tasks?: boolean
): { to: string; helper: string; content: string } {
  if (resource) {
    return {
      to: cfg.getIntegrationStatusRoute(integration.kind, integration.name),
      helper: 'Back to integration',
      content: resource.toUpperCase(),
    };
  }

  if (tasks) {
    return {
      to: cfg.getIntegrationStatusRoute(integration.kind, integration.name),
      helper: 'Back to integration',
      content: 'Pending Tasks',
    };
  }

  return {
    to: cfg.routes.integrations,
    helper: 'Back to integrations',
    content: integration.name,
  };
}
