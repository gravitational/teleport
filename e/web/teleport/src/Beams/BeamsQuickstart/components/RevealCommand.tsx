import { ReactNode, useId, useState } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';

import { CodeBlock } from './CodeBlock';

export function RevealCommand({
  hint,
  command,
}: {
  hint: ReactNode;
  command: string;
}) {
  const [open, setOpen] = useState(false);
  const contentId = useId();

  return (
    <Flex flexDirection="column" gap={3}>
      <HintRow>
        {hint}{' '}
        <LinkButton
          type="button"
          aria-expanded={open}
          aria-controls={contentId}
          onClick={() => setOpen(o => !o)}
        >
          {open ? 'Hide command.' : 'Show command.'}
        </LinkButton>
      </HintRow>
      <div id={contentId} hidden={!open}>
        {open && <CodeBlock command={command} />}
      </div>
    </Flex>
  );
}

const HintRow = styled.div`
  ${p => p.theme.typography.subtitle2};
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const LinkButton = styled.button`
  background: none;
  border: none;
  padding: 0;
  font: inherit;
  font-weight: ${p => p.theme.fontWeights.bold};
  color: ${p => p.theme.colors.brand};
  cursor: pointer;
  &:hover {
    text-decoration: underline;
  }
`;
