import React from 'react';
import { Flex, Text } from 'design';

import { AccessRequestCheckoutButton } from 'e-teleterm/ui/StatusBar/AccessRequestCheckoutButton';

import { useActiveDocumentClusterBreadcrumbs } from './useActiveDocumentClusterBreadcrumbs';
import { ShareFeedback } from './ShareFeedback';

export function StatusBar() {
  const clusterBreadcrumbs = useActiveDocumentClusterBreadcrumbs();

  return (
    <Flex
      width="100%"
      height="28px"
      css={`
        border-top: 1px solid ${props => props.theme.colors.primary.light};
      `}
      alignItems="center"
      justifyContent="space-between"
      px={2}
      overflow="hidden"
    >
      <Text
        color="text.secondary"
        fontSize="14px"
        css={`
          white-space: nowrap;
        `}
        title={clusterBreadcrumbs}
      >
        {clusterBreadcrumbs}
      </Text>
      <Flex gap={2} alignItems="center">
        <AccessRequestCheckoutButton />
        <ShareFeedback />
      </Flex>
    </Flex>
  );
}
