import { Box, Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';

export const UpdatedAtDisplay = ({
  theme,
  usageUpdatedAt,
  usageUpdatedAtFormatted,
}: {
  theme: any;
  usageUpdatedAt: number;
  usageUpdatedAtFormatted: string;
}) => {
  return (
    <Text
      color={theme.colors.text.slightlyMuted}
      style={{ fontStyle: 'italic' }}
      data-testid="updated-at-display"
    >
      {usageUpdatedAt > 0 ? (
        <Flex alignItems="center">
          <Box mr="2">Last updated: {usageUpdatedAtFormatted}</Box>
          <IconTooltip children="Updated every 12 hours." />
        </Flex>
      ) : (
        'Updated every 12 hours'
      )}
    </Text>
  );
};
