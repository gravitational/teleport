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

import getSsoIcon from 'teleport/AuthConnectors/ssoIcons/getSsoIcon';
import { State as ResourceState } from 'teleport/components/useResources';

import { AuthConnectorTile, LocalConnectorTile } from '../AuthConnectorTile';
import { AuthConnectorsGrid } from '../ConnectorList/ConnectorList';

export default function EmptyList({ onCreate, isLocalDefault }: Props) {
  return (
    <AuthConnectorsGrid>
      <LocalConnectorTile isDefault={isLocalDefault} />
      <AuthConnectorTile
        key="github-placeholder"
        kind="github"
        id="github-placeholder"
        name={'GitHub'}
        Icon={getSsoIcon('github')}
        isDefault={false}
        isPlaceholder={true}
        onSetup={() => onCreate('github')}
      />
    </AuthConnectorsGrid>
  );
}

type Props = {
  onCreate: ResourceState['create'];
  isLocalDefault: boolean;
};
