import {
  Box,
  Flex,
  Text,
  WarningCircleIcon,
} from '@gravitational/design-system';
import { ReactNode } from 'react';

export function EditableInput({
  title,
  content,
  error,
  muted = false,
}: {
  title: string;
  content?: ReactNode;
  error?: Error;
  muted?: boolean;
}) {
  return (
    <Flex
      pl={1}
      ml={-1}
      align="center"
      backgroundColor={error && 'interactive.tonal.danger.0'}
    >
      <Text
        as="div"
        textStyle="body2"
        color={muted ? 'text.muted' : undefined}
        fontWeight="bold"
        width="200px"
        height="38px"
        alignContent="center"
      >
        {title}:
      </Text>
      <Box
        whiteSpace="pre"
        textWrap="wrap"
        wordBreak="break-all"
        margin={0}
        minWidth="200px"
      >
        {content}
      </Box>
    </Flex>
  );
}

export function ErrorTooltipIcon() {
  return (
    <WarningCircleIcon
      role="graphics-symbol"
      aria-label="Error"
      boxSize="18px"
      color="interactive.solid.danger.default"
    />
  );
}
