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

import { useHistory } from 'react-router';
import styled from 'styled-components';

import { Box } from 'design';

import { State as ResourceState } from 'teleport/components/useResources';
import cfg from 'teleport/config';
import { Resource } from 'teleport/services/resources';

import { AuthConnectorTile, LocalConnectorTile } from '../AuthConnectorTile';
import getSsoIcon from '../ssoIcons/getSsoIcon';

export default function ConnectorList({ items, onDelete }: Props) {
  const history = useHistory();
  items = items || [];
  const $items = items.map(item => {
    const { id, name, kind } = item;

    const Icon = getSsoIcon(kind, name);

    return (
      <AuthConnectorTile
        key={id}
        kind={kind}
        id={id}
        Icon={Icon}
        isDefault={false}
        isPlaceholder={false}
        onEdit={() => history.push(cfg.getEditAuthConnectorRoute(kind, name))}
        onDelete={onDelete}
        name={name}
      />
    );
  });

  return (
    <AuthConnectorsGrid>
      <LocalConnectorTile />
      {$items}
    </AuthConnectorsGrid>
  );
}

type Props = {
  items: Resource<'github'>[];
  onDelete: ResourceState['remove'];
};

export const AuthConnectorsGrid = styled(Box)`
  width: 100%;
  display: grid;
  gap: ${p => p.theme.space[3]}px;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
`;
