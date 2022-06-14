import React, { useRef, useState } from 'react';
import { ButtonIcon, Flex, Popover, Text } from 'design';
import { ChatBubble } from 'design/Icon';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ShareFeedback } from './ShareFeedback/ShareFeedback';

export function StatusBar() {
  const ctx = useAppContext();
  const shareButtonRef = useRef<HTMLButtonElement>();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);

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
      <ButtonIcon
        setRef={shareButtonRef}
        title="Share feedback"
        size={0}
        onClick={() => setIsPopoverOpened(true)}
      >
        <ChatBubble fontSize="14px" />
      </ButtonIcon>
      <Popover
        open={isPopoverOpened}
        anchorEl={shareButtonRef.current}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
        transformOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        onClose={() => setIsPopoverOpened(false)}
      >
        <ShareFeedback onClose={() => setIsPopoverOpened(false)} />
      </Popover>
    </Flex>
  );
}
