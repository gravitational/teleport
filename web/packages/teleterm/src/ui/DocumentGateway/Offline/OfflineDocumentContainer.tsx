import React from 'react';

import { Flex } from 'design';

export function OfflineDocumentContainer(
  props: React.PropsWithChildren<unknown>
) {
  return (
    <Flex
      maxWidth="590px"
      width="100%"
      flexDirection="column"
      mx="auto"
      alignItems="center"
      mt={11}
    >
      {props.children}
    </Flex>
  );
}
