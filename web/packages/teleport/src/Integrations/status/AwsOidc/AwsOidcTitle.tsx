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
import { useHistory } from 'react-router';
import { Link as InternalLink } from 'react-router-dom';
import { useTheme } from 'styled-components';

import { ButtonIcon, Flex, Label, Link, MenuItem, Text } from 'design';
import * as Icons from 'design/Icon';
import { ArrowLeft } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { MenuButton } from 'shared/components/MenuAction';

import cfg from 'teleport/config';
import { getStatusAndLabel } from 'teleport/Integrations/helpers';
import {
  IntegrationOperations,
  useIntegrationOperation,
} from 'teleport/Integrations/Operations';
import type { EditableIntegrationFields } from 'teleport/Integrations/Operations/useIntegrationOperation';
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
  const theme = useTheme();
  const history = useHistory();
  const integrationOps = useIntegrationOperation();
  const { status, labelKind } = getStatusAndLabel(integration);
  const content = getContent(integration, resource, tasks);

  async function removeIntegration() {
    await integrationOps.remove();
    integrationOps.clear();
    history.push(cfg.routes.integrations);
  }

  async function editIntegration(req: EditableIntegrationFields) {
    await integrationOps.edit(req);
    integrationOps.clear();
  }

  return (
    <Flex justifyContent="space-between">
      <Flex alignItems="center" data-testid="aws-oidc-title">
        <HoverTooltip placement="bottom" tipContent={content.helper}>
          <ButtonIcon as={InternalLink} to={content.to} aria-label="back">
            <ArrowLeft size="medium" />
          </ButtonIcon>
        </HoverTooltip>
        <Flex flexDirection="column" mx={2}>
          <Text bold fontSize={6}>
            {content.content}
          </Text>
          <Flex gap={1}>
            Role ARN:{' '}
            <Link
              target="_blank"
              href={`https://console.aws.amazon.com/iamv2/home#/roles/details/${integration.name}`}
            >
              <Text
                style={{
                  fontFamily: theme.fonts.mono,
                }}
              >
                {integration.spec?.roleArn}
              </Text>
            </Link>
          </Flex>
        </Flex>
        <Label kind={labelKind} aria-label="status" px={3}>
          {status}
        </Label>
      </Flex>
      {!resource && !tasks && (
        <MenuButton icon={<Icons.Cog size="small" />}>
          <MenuItem onClick={() => integrationOps.onEdit(integration)}>
            Edit...
          </MenuItem>
          <MenuItem onClick={() => integrationOps.onRemove(integration)}>
            Delete...
          </MenuItem>
        </MenuButton>
      )}
      <IntegrationOperations
        operation={integrationOps.type}
        integration={integrationOps.item}
        close={integrationOps.clear}
        edit={editIntegration}
        remove={removeIntegration}
      />
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
