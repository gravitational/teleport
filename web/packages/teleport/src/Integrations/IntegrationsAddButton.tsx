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

import { Button } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';

import cfg from 'teleport/config';

export function IntegrationsAddButton({
  requiredPermissions,
}: {
  requiredPermissions: { value: boolean; label: string }[];
}) {
  const canCreateIntegrations = requiredPermissions.some(v => v.value);
  const missingPermissions = requiredPermissions
    .filter(perm => !perm.value)
    .map(perm => perm.label);

  return (
    <HoverTooltip
      placement="bottom"
      tipContent={
        canCreateIntegrations ? null : (
          <MissingPermissionsTooltip
            requiresAll={false}
            missingPermissions={missingPermissions}
          />
        )
      }
    >
      <Button
        intent="primary"
        fill="border"
        as={Link}
        ml="auto"
        width="240px"
        disabled={!canCreateIntegrations}
        to={cfg.getIntegrationEnrollRoute()}
        title={
          canCreateIntegrations
            ? ''
            : 'You do not have access to add new integrations'
        }
      >
        Enroll New Integration
      </Button>
    </HoverTooltip>
  );
}
