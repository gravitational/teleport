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

import React, { PropsWithChildren } from 'react';
import { useTheme } from 'styled-components';

import { FeatureNames } from 'design/constants';

import { ToolTipBadge } from 'teleport/components/ToolTipBadge';

type Props = {
  borderRadius?: number;
  badgeTitle?: BadgeTitle;
  sticky?: boolean;
};

export const ToolTipNoPermBadge: React.FC<PropsWithChildren<Props>> = ({
  children,
  borderRadius = 2,
  badgeTitle = BadgeTitle.LackingPermissions,
  sticky = false,
}) => {
  const theme = useTheme();

  return (
    <ToolTipBadge
      borderRadius={borderRadius}
      badgeTitle={badgeTitle}
      sticky={sticky}
      color={theme.colors.error.main}
    >
      {children}
    </ToolTipBadge>
  );
};

export enum BadgeTitle {
  LackingPermissions = 'Lacking Permissions',
  LackingEnterpriseLicense = 'Enterprise Only',
  LackingIgs = `${FeatureNames.IdentityGovernance} Only`,
}
