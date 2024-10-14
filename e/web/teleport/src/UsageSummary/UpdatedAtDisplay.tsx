import React from 'react';

import { Box, Flex, Text } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { displayDateTime } from 'design/datetime';

export const UpdatedAtDisplay = ({
  theme,
  usageUpdatedAt,
}: {
  theme: any;
  usageUpdatedAt: number;
}) => {
  return (
    <Text
      color={theme.colors.text.slightlyMuted}
      style={{ fontStyle: 'italic' }}
      data-testid="updated-at-display"
    >
      {usageUpdatedAt > 0 ? (
        <Flex alignItems="center">
          <Box mr="2">Last updated: {displayUnixDateTime(usageUpdatedAt)}</Box>
          <ToolTipInfo children="Updated every 12 hours." />
        </Flex>
      ) : (
        'Updated every 12 hours'
      )}
    </Text>
  );
};

export function displayUnixDateTime(seconds: number) {
  // Multiply by 1000 b/c date constructor expects milliseconds.
  return displayDateTime(new Date(seconds * 1000));
}
