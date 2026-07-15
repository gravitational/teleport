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

import {
  BadgeTitle,
  ToolTipNoPermBadge,
} from 'teleport/components/ToolTipNoPermBadge';

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
