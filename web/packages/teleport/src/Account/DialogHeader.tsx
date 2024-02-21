import Box from 'design/Box';
import Text from 'design/Text';
import React from 'react';

export function DialogHeader({
  stepIndex,
  flowLength,
  title,
}: {
  stepIndex: number;
  flowLength: number;
  title: string;
}) {
  return (
    <Box mb={4}>
      <Text typography="body1">
        Step {stepIndex + 1} of {flowLength}
      </Text>
      <Text typography="h4">{title}</Text>
    </Box>
  );
}
