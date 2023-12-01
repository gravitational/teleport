/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { PropsWithChildren } from 'react';
import { useTheme } from 'styled-components';

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
}
