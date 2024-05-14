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

import {
  Icon,
  NavigationItemSize,
  SmallIcon,
} from 'teleport/Navigation/common';

import type { TeleportFeature } from 'teleport/types';

export function getIcon(feature: TeleportFeature, size: NavigationItemSize) {
  switch (size) {
    case NavigationItemSize.Large:
      return <Icon>{<feature.navigationItem.icon />}</Icon>;

    case NavigationItemSize.Small:
      return <SmallIcon>{<feature.navigationItem.icon />}</SmallIcon>;
  }
}
