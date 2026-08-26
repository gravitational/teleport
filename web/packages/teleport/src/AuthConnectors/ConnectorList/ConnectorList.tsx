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
import {
  DefaultAuthConnector,
  KindAuthConnectors,
  Resource,
} from 'teleport/services/resources';

import { AuthConnectorTile, LocalConnectorTile } from '../AuthConnectorTile';
import getSsoIcon from '../ssoIcons/getSsoIcon';

export function ConnectorList<T extends KindAuthConnectors>({
  items,
  defaultConnector,
  setAsDefault,
  onDelete,
}: Props<T>) {
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
        isDefault={
          defaultConnector.name === name && defaultConnector.type === kind
        }
        onSetAsDefault={() => setAsDefault({ type: kind, name })}
        isPlaceholder={false}
        onEdit={() => history.push(cfg.getEditAuthConnectorRoute(kind, name))}
        onDelete={onDelete}
        name={name}
      />
    );
  });

  return (
    <AuthConnectorsGrid>
      <LocalConnectorTile
        isDefault={defaultConnector.type === 'local'}
        setAsDefault={() => setAsDefault({ type: 'local' })}
      />
      {$items}
    </AuthConnectorsGrid>
  );
}

type Props<T extends KindAuthConnectors> = {
  items: Resource<T>[];
  defaultConnector: DefaultAuthConnector;
  setAsDefault: (defaultConnector: DefaultAuthConnector) => void;
  onDelete: ResourceState['remove'];
};

export const AuthConnectorsGrid = styled(Box)`
  width: 100%;
  display: grid;
  gap: ${p => p.theme.space[3]}px;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
`;
