import React from 'react';
import { Text, Flex, ButtonPrimary } from 'design';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { AuthProviderType } from 'shared/services';
import { State as ResourceState } from 'teleport/components/useResources';

import getSsoIcon from './../getSsoIcon';

export default function ConnectorListItem({
  name,
  kind,
  id,
  onEdit,
  onDelete,
  showAuthConnectorsCTA,
}: Props) {
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);
  const { desc, SsoIcon } = getSsoIcon(kind, showAuthConnectorsCTA);

  const iconProps: any = {
    fontSize: '48px',
    mb: 3,
    mt: 3,
  };

  return (
    <Flex
      width="240px"
      height="240px"
      borderRadius="3"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      bg="levels.surface"
      px="5"
      pt="2"
      pb="5"
      mb={4}
      mr={5}
      css={`
        position: relative;
        box-shadow: ${props => props.theme.boxShadow[1]};
      `}
    >
      <Flex width="100%" justifyContent="center">
        <MenuIcon buttonIconProps={menuActionProps}>
          <MenuItem onClick={onClickDelete}>Delete...</MenuItem>
        </MenuIcon>
      </Flex>
      <Flex
        flex="1"
        mb="3"
        alignItems="center"
        justifyContent="center"
        flexDirection="column"
        width="200px"
        style={{ textAlign: 'center' }}
      >
        <SsoIcon {...iconProps} />
        <Text style={{ width: '100%' }} typography="body2" bold caps mb="1">
          {name}
        </Text>
        <Text style={{ width: '100%' }} typography="body2" color="text.main">
          {desc}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" size="medium" block onClick={onClickEdit}>
        EDIT CONNECTOR
      </ButtonPrimary>
    </Flex>
  );
}

const menuActionProps = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};

type Props = {
  name: string;
  id: string;
  kind: AuthProviderType;
  onEdit: ResourceState['edit'];
  onDelete: ResourceState['remove'];
  showAuthConnectorsCTA: boolean;
};
