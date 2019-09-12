import React from 'react';
import styled from 'styled-components';
import { Text, Flex, ButtonPrimary } from 'design';
import * as Icons from 'design/Icon';
import ActionMenu, {
  MenuItem,
} from 'shared/components/ActionMenu';

export default function TrustedListItem({
  name,
  id,
  onEdit,
  onDelete,
  ...rest
}) {
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);

  return (
    <StyledItem
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
        <Text typography="h4" caps bold>
          {name}
        </Text>
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
        <Icons.Link
          style={{ textAlign: 'center' }}
          fontSize="80px"
          color="text.primary"
        />
      </Flex>
      <ButtonPrimary mt="auto" px="1" size="medium" block onClick={onClickEdit}>
        EDIT TRUSTED CLUSTER
      </ButtonPrimary>
    </StyledItem>
  );
}

const StyledItem = styled(Flex)`
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
