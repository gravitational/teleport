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

import { ButtonIcon, H2 } from 'design';
import { DialogHeader } from 'design/Dialog';
import * as icons from 'design/Icon';

import { RootClusterUri, routing } from 'teleterm/ui/uri';

export function CommonHeader(props: {
  onCancel(): void;
  rootClusterUri: RootClusterUri;
}) {
  const rootClusterName = routing.parseClusterName(props.rootClusterUri);

  return (
    <DialogHeader justifyContent="space-between" mb={0} alignItems="baseline">
      <H2 mb={4}>
        Unlock hardware key to access <strong>{rootClusterName}</strong>
      </H2>

      <ButtonIcon
        type="button"
        onClick={props.onCancel}
        color="text.slightlyMuted"
      >
        <icons.Cross size="medium" />
      </ButtonIcon>
    </DialogHeader>
  );
}
