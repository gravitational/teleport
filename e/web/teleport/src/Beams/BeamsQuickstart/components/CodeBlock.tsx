import { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';
import { ButtonBorder } from 'design/Button';
import { copyToClipboard } from 'design/utils/copyToClipboard';

const RESET_COPIED_MS = 1500;

export function CodeBlock({
  command,
  prompt = '$',
}: {
  command: string;
  prompt?: string;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  async function onCopy() {
    await copyToClipboard(command);
    setCopied(true);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), RESET_COPIED_MS);
  }

  return (
    <Block>
      <Command>
        {prompt && <Prompt>{prompt}</Prompt>}
        <CommandText>{command}</CommandText>
      </Command>
      <CopyButton $copied={copied} size="small" onClick={onCopy}>
        {copied ? 'Copied!' : 'Copy'}
      </CopyButton>
    </Block>
  );
}

export const InlineCode = styled.code`
  ${p => p.theme.typography.subtitle2};
  font-family: ${p => p.theme.fonts.mono};
  color: ${p => p.theme.colors.interactive.solid.alert.default};
  background: ${p => p.theme.colors.levels.elevated};
  padding: 0 ${p => p.theme.space[1]}px;
  border-radius: ${p => p.theme.radii[1]}px;
`;

const Block = styled(Flex).attrs({
  alignItems: 'flex-start',
  gap: 2,
  p: 2,
  pl: 3,
})`
  background: ${p => p.theme.colors.levels.elevated};
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${p => p.theme.radii[2]}px;
`;

const Command = styled(Flex).attrs({ gap: 1, pt: 1, flex: 1 })`
  ${p => p.theme.typography.subtitle2};
  font-family: ${p => p.theme.fonts.mono};
  color: ${p => p.theme.colors.text.main};
  min-width: 0;
  white-space: pre-wrap;
  word-break: break-word;
`;

const Prompt = styled.span`
  user-select: none;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const CommandText = styled.span`
  flex: 1;
  min-width: 0;
`;

const CopyButton = styled(ButtonBorder)<{ $copied: boolean }>`
  flex-shrink: 0;
  /* Needed, to keep the button on click styling across hover/focus/active 
    so the visual confirmation doesn't disappear while the cursor sits over the button. */
  ${p =>
    p.$copied &&
    `
    &, &:hover, &:focus, &:active {
      color: ${p.theme.colors.brand};
      border-color: ${p.theme.colors.brand};
      background-color: transparent;
    }
  `}
`;
