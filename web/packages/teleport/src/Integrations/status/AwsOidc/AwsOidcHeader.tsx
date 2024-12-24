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

import { ButtonIcon, ButtonText, Flex, Text } from 'design';
import { Plugs } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import { Integration } from 'teleport/services/integrations';

export function AwsOidcHeader({
  integration,
  resource = undefined,
}: {
  integration: Integration;
  resource?: AwsResource;
}) {
  const history = useHistory();
  const divider = (
    <Text typography="body3" color="text.slightlyMuted">
      /
    </Text>
  );

  return (
    <Flex
      alignItems="center"
      borderBottom={1}
      borderColor="levels.surface"
      width={'100%'}
      pl={5}
      py={1}
      data-testid="aws-oidc-header"
    >
      <HoverTooltip position="bottom" tipContent="Back to Integrations">
        <ButtonIcon
          size="small"
          as={InternalLink}
          to={cfg.routes.integrations}
          aria-label="integrations-table"
          color="text.slightlyMuted"
        >
          <Plugs size="small" />
        </ButtonIcon>
      </HoverTooltip>
      {!resource ? (
        <>
          {divider}
          <Text typography="body3" color="text.slightlyMuted" ml={2}>
            {integration.name}
          </Text>
        </>
      ) : (
        <>
          {divider}
          <ButtonText
            size="small"
            onClick={() =>
              history.push(
                cfg.getIntegrationStatusRoute(
                  integration.kind,
                  integration.name
                )
              )
            }
          >
            {integration.name}
          </ButtonText>
          {divider}
          <Text typography="body3" color="text.slightlyMuted" ml={2}>
            {resource.toUpperCase()}
          </Text>
        </>
      )}
    </Flex>
  );
}
