import React from 'react';
import { Flex, Text } from 'design';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ShareFeedback } from './ShareFeedback';

export function StatusBar() {
  const ctx = useAppContext();
  ctx.workspacesService.useState();

  return (
    <Flex
      width="100%"
      height="28px"
      bg="primary.dark"
      alignItems="center"
      justifyContent="space-between"
      px={2}
      overflow="hidden"
    >
      {/*TODO (gzdunek) display proper info here */}
      <Text color="text.secondary" fontSize="14px">
        {ctx.workspacesService.getRootClusterUri()}
      </Text>
      <ShareFeedback />
    </Flex>
  );
}
