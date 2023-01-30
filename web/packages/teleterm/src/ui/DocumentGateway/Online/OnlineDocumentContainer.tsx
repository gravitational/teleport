import React from 'react';

import { Box } from 'design';

export function OnlineDocumentContainer(
  props: React.PropsWithChildren<unknown>
) {
  return (
    <Box maxWidth="590px" width="100%" mx="auto" mt="4" px="5">
      {props.children}
    </Box>
  );
}
