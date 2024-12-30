import styled from 'styled-components';

import Box from 'design/Box';

import { State as ResourceState } from 'teleport/components/useResources';
import {
  AuthConnectorTile,
  LocalConnectorTile,
} from 'teleport/AuthConnectors/AuthConnectorTile';
import getSsoIcon from 'teleport/AuthConnectors/ssoIcons/getSsoIcon';

import { State as AuthConnectorState } from '../useAuthConnectors';

export default function ConnectorList({ items, onEdit, onDelete }: Props) {
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
        onEdit={onEdit}
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
  items: AuthConnectorState['items'];
  onEdit: ResourceState['edit'];
  onDelete: ResourceState['remove'];
};

export const AuthConnectorsGrid = styled(Box)`
  width: 100%;
  display: grid;
  gap: ${p => p.theme.space[3]}px;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
`;
