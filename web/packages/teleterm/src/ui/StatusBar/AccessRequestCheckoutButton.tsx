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

import { ButtonPrimary, Text } from 'design';
import { ListAddCheck } from 'design/Icon';

import { useAccessRequestsButton } from 'teleterm/ui/StatusBar/useAccessRequestCheckoutButton';

export function AccessRequestCheckoutButton() {
  const { toggleAccessRequestBar, getAddedItemsCount, isCollapsed } =
    useAccessRequestsButton();
  const count = getAddedItemsCount();

  if (count > 0 && isCollapsed()) {
    return (
      <ButtonPrimary
        onClick={toggleAccessRequestBar}
        px={2}
        size="small"
        title="Toggle Access Request Checkout"
      >
        <ListAddCheck mr={2} size="small" color="buttons.primary.text" />
        <Text fontSize="12px">{count}</Text>
      </ButtonPrimary>
    );
  }
  return null;
}
