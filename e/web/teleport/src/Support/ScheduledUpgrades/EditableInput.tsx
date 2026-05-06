import { ReactNode } from 'react';
import { useTheme } from 'styled-components';

import { Box, Flex, Text } from 'design';

export function EditableInput({
  title,
  content,
  error,
  muted = false,
}: {
  title: string;
  content?: ReactNode;
  error: Error;
  muted?: boolean;
}) {
  const theme = useTheme();
  return (
    <Flex
      pl={1}
      ml={-1}
      alignItems="center"
      backgroundColor={error && theme.colors.interactive.tonal.danger[0]}
    >
      <Text
        typography="body2"
        color={muted ? 'text.muted' : undefined}
        bold
        style={{ width: '200px', height: '38px', alignContent: 'center' }}
      >
        {title}:
      </Text>
      <Box
        style={{
          whiteSpace: 'pre',
          textWrap: 'wrap',
          wordBreak: 'break-all',
          margin: 0,
          minWidth: '200px',
        }}
      >
        {content}
      </Box>
    </Flex>
  );
}
