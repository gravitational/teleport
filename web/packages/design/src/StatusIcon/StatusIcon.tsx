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

import React from 'react';
import { useTheme } from 'styled-components';

import * as Icon from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';

export type StatusKind = 'neutral' | 'danger' | 'info' | 'warning' | 'success';

export const StatusIcon = ({
  kind,
  customIcon: CustomIcon,
  ...otherProps
}: {
  kind: StatusKind;
  customIcon?: React.ComponentType<IconProps>;
} & IconProps) => {
  const commonProps = { role: 'graphics-symbol', ...otherProps };
  const theme = useTheme();

  if (CustomIcon) {
    return <CustomIcon {...commonProps} />;
  }
  switch (kind) {
    case 'success':
      return (
        <Icon.Checks
          color={theme.colors.interactive.solid.success.default}
          aria-label="Success"
          {...commonProps}
        />
      );
    case 'danger':
      return (
        <Icon.WarningCircle
          color={theme.colors.interactive.solid.danger.default}
          aria-label="Danger"
          {...commonProps}
        />
      );
    case 'info':
      return (
        <Icon.Info
          color={theme.colors.interactive.solid.accent.default}
          aria-label="Info"
          {...commonProps}
        />
      );
    case 'warning':
      return (
        <Icon.Warning
          color={theme.colors.interactive.solid.alert.default}
          aria-label="Warning"
          {...commonProps}
        />
      );
    case 'neutral':
      return <Icon.Notification aria-label="Note" {...commonProps} />;
  }
};
