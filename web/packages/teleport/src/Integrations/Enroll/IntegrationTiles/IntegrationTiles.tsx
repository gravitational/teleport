/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Link } from 'react-router-dom';

import { Flex, Text } from 'design';
import { ResourceIconName } from 'design/ResourceIcon';

import {
  BadgeTitle,
  ToolTipNoPermBadge,
} from 'teleport/components/ToolTipNoPermBadge';
import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';

import { IntegrationIcon, IntegrationTile } from '../common';

export function renderExternalAuditStorageBadge(
  hasExternalAuditStorageAccess: boolean,
  isEnterprise: boolean
) {
  if (!isEnterprise)
    return (
      <ToolTipNoPermBadge badgeTitle={BadgeTitle.LackingEnterpriseLicense}>
        <div>Unlock External Audit Storage with Teleport Enterprise</div>
      </ToolTipNoPermBadge>
    );

  return !hasExternalAuditStorageAccess ? (
    <GenericNoPermBadge
      noAccess={!hasExternalAuditStorageAccess}
      kind="External Audit Storage"
    />
  ) : undefined;
}

export function GenericNoPermBadge({
  noAccess,
  kind = 'integration',
}: {
  noAccess: boolean;
  kind?: string;
}) {
  if (noAccess) {
    return (
      <ToolTipNoPermBadge>
        <div>
          You don’t have sufficient permissions to create an {kind}. Reach out
          to your Teleport administrator to request additional permissions.
        </div>
      </ToolTipNoPermBadge>
    );
  }
}

export function GenericIntegrationTile({
  kind,
  hasAccess,
  name,
  icon,
}: {
  kind: IntegrationKind;
  hasAccess: boolean;
  name: string;
  icon: ResourceIconName;
}) {
  return (
    <IntegrationTile
      disabled={!hasAccess}
      as={hasAccess ? Link : null}
      to={hasAccess ? cfg.getIntegrationEnrollRoute(kind) : null}
      data-testid={`tile-${kind}`}
    >
      <Flex flexBasis={100}>
        <IntegrationIcon name={icon} size={80} />
      </Flex>
      <Flex>
        <Text>{name}</Text>
      </Flex>
      <GenericNoPermBadge noAccess={!hasAccess} />
    </IntegrationTile>
  );
}
