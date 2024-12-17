import { Flex } from 'design';
import { AuthProviderType } from 'shared/services';
import { State as ResourceState } from 'teleport/components/useResources';

import { State as AuthConnectorsState } from '../useAuthConnectors';

import ConnectorListItem from './ConnectorListItem';

export default function ConnectorList({
  items,
  onEdit,
  onDelete,
  showAuthConnectorsCTA,
}: Props) {
  items = items || [];
  const $items = items.map(item => {
    const { id, name, kind } = item;
    return (
      <ConnectorListItem
        key={id}
        id={id}
        onEdit={onEdit}
        onDelete={onDelete}
        name={name}
        kind={kind as AuthProviderType}
        showAuthConnectorsCTA={showAuthConnectorsCTA}
      />
    );
  });

  return (
    <Flex flexWrap="wrap" alignItems="center" flex={1} gap={5}>
      {$items}
    </Flex>
  );
}

type Props = {
  items: AuthConnectorsState['items'];
  onEdit: ResourceState['edit'];
  onDelete: ResourceState['remove'];
  showAuthConnectorsCTA: boolean;
};
