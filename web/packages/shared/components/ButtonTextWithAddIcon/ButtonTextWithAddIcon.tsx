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

import React from 'react';
import { ButtonText } from 'design';
import { Add as AddIcon } from 'design/Icon';

export const ButtonTextWithAddIcon = ({
  label,
  onClick,
  disabled,
  iconSize = 12,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  iconSize?: number | 'small' | 'medium' | 'large' | 'extraLarge';
}) => {
  return (
    <ButtonText onClick={onClick} disabled={disabled} compact>
      <AddIcon
        className="icon-add"
        size={iconSize}
        css={`
          margin-right: 3px;
        `}
      />
      {label}
    </ButtonText>
  );
};
