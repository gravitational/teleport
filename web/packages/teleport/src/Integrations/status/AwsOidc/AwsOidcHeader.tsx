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
import { ArrowLeft, ChevronRight } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import Link from 'design/Link';

import cfg from 'teleport/config';
import { getStatusAndLabel } from 'teleport/Integrations/helpers';
import { Integration } from 'teleport/services/integrations';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';

export function AwsOidcHeader({
  integration,
  resource = undefined,
}: {
  integration: Integration;
  resource?: AwsResource;
}) {
  const { status, labelKind } = getStatusAndLabel(integration);
  return (
    <Flex alignItems="center">
      <HoverTooltip position="bottom" tipContent="Back to Integrations">
        <ButtonIcon
          as={InternalLink}
          to={cfg.routes.integrations}
          aria-label="back"
        >
          <ArrowLeft size="medium" />
        </ButtonIcon>
      </HoverTooltip>
      {!resource ? (
        <Text bold fontSize={6} mx={2}>
          {integration.name}
        </Text>
      ) : (
        <>
          <Link
            color="text.main"
            href={cfg.getIntegrationStatusRoute(
              integration.kind,
              integration.name
            )}
          >
            <Text bold fontSize={6} mx={2}>
              {integration.name}
            </Text>
          </Link>
          <ChevronRight />
          <Text bold fontSize={6}>
            {resource.toUpperCase()}
          </Text>
        </>
      )}
      <Label kind={labelKind} aria-label="status" px={3} ml={3}>
        {status}
      </Label>
    </Flex>
  );
}
