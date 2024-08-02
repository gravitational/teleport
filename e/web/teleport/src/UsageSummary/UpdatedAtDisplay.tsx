import React from 'react';
import { format } from 'date-fns';

import cfg from 'shared/config';
import { Box, Flex, Text } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';

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
  return format(new Date(seconds * 1000), cfg.dateTimeFormat);
}
