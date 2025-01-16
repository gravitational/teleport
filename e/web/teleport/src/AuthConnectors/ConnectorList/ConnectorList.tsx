import { useHistory } from 'react-router';
import styled from 'styled-components';

import Box from 'design/Box';

import cfg from 'e-teleport/config';
import {
  AuthConnectorTile,
  LocalConnectorTile,
} from 'teleport/AuthConnectors/AuthConnectorTile';
import getSsoIcon from 'teleport/AuthConnectors/ssoIcons/getSsoIcon';
import { State as ResourceState } from 'teleport/components/useResources';
import { KindAuthConnectors, Resource } from 'teleport/services/resources';

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
        onEdit={() =>
          history.push(cfg.oss.getEditAuthConnectorRoute(kind, name))
        }
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
  items: Resource<KindAuthConnectors>[];
  onDelete: ResourceState['remove'];
};

export const AuthConnectorsGrid = styled(Box)`
  width: 100%;
  display: grid;
  gap: ${p => p.theme.space[3]}px;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
`;
