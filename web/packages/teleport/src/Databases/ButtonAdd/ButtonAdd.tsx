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
import { ButtonPrimary } from 'design';

export default function ButtonAdd(props: Props) {
  const { canCreate, isLeafCluster, onClick } = props;
  const disabled = isLeafCluster || !canCreate;

  let title = '';
  if (!canCreate) {
    title = 'You do not have access to add a database';
  }

  if (isLeafCluster) {
    title = 'Adding a database to a leaf cluster is not supported';
  }

  return (
    <ButtonPrimary
      title={title}
      disabled={disabled}
      width="240px"
      onClick={onClick}
    >
      Add Database
    </ButtonPrimary>
  );
}

type Props = {
  isLeafCluster: boolean;
  canCreate: boolean;
  onClick?: () => void;
};
