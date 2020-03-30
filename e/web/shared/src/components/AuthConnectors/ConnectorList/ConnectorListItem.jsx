import React from 'react';
import styled from 'styled-components';
import { Text, Flex, ButtonPrimary } from 'design';
import ActionMenu, { MenuItem } from 'shared/components/ActionMenu';
import getSsoIcon from './../getSsoIcon';

export default function ConnectorListItem({
  name,
  kind,
  id,
  onEdit,
  onDelete,
  ...rest
}) {
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);
  const { desc, SsoIcon } = getSsoIcon(kind);

  const iconProps = {
    fontSize: '68px',
    mb: 3,
    mt: 3,
  };

  if (kind === 'saml') {
    iconProps.width = '160px';
    iconProps.height = '50px';
    iconProps.mt = 5;
  }

  return (
    <StyledConnectorListItem
      width="260px"
      height="260px"
      borderRadius="3"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      bg="primary.light"
      px="5"
      pt="4"
      pb="5"
      {...rest}
    >
      <Flex width="100%" justifyContent="center">
        <ActionMenu buttonIconProps={menuActionProps}>
          <MenuItem onClick={onClickDelete}>Delete...</MenuItem>
        </ActionMenu>
      </Flex>
      <Flex
        flex="1"
        mb="3"
        alignItems="center"
        justifyContent="center"
        flexDirection="column"
      >
        <SsoIcon {...iconProps} />
        <Text typography="body2" bold caps="uppercase" mb="1">
          {name}
        </Text>
        <Text typography="body2" color="text.primary">
          {desc}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" size="medium" block onClick={onClickEdit}>
        EDIT CONNECTOR
      </ButtonPrimary>
    </StyledConnectorListItem>
  );
}

const StyledConnectorListItem = styled(Flex)`
  position: relative;
  transition: all 0.3s;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  &:hover {
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.56);
  }
`;

const menuActionProps = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};
