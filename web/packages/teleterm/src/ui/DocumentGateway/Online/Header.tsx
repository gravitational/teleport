import React from 'react';

import { ButtonSecondary, Flex, Text } from 'design';

interface HeaderProps {
  onClose: () => void;
}

export function Header(props: HeaderProps) {
  return (
    <Flex justifyContent="space-between" mb="4" flexWrap="wrap" gap={2}>
      <Text typography="h3" color="text.secondary">
        Database Connection
      </Text>
      <ButtonSecondary size="small" onClick={props.onClose}>
        Close Connection
      </ButtonSecondary>
    </Flex>
  );
}
