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

import { ButtonText } from 'design';
import { ButtonProps } from 'design/Button';
import { Add as AddIcon } from 'design/Icon';
import type { IconSize } from 'design/Icon/Icon';

export const ButtonWithAddIcon = ({
  Button = ButtonText,
  label,
  iconSize = 12,
  compact = true,
  pr = 2,
  ...props
}: ButtonProps<'button'> & {
  Button?: React.ComponentType<ButtonProps<'button'>>;
  label: string;
  iconSize?: IconSize;
}) => {
  return (
    <Button {...props} compact={compact} pr={pr}>
      <AddIcon
        className="icon-add"
        size={iconSize}
        css={`
          margin-right: 3px;
        `}
      />
      {label}
    </Button>
  );
};
