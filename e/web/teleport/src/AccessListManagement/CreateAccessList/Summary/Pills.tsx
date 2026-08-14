import { useState } from 'react';
import styled from 'styled-components';

import { Flex, Stack, Status } from 'design';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

const Pill = ({ children }) => (
  <Status
    kind="neutral"
    variant="filled-subtle"
    icon={false}
    css={`
      max-width: 100%;
    `}
  >
    <HoverTooltip tipContent={children} showOnlyOnOverflow>
      <PillText>{children}</PillText>
    </HoverTooltip>
  </Status>
);

export const Pills = ({ texts }: { texts: string[] }) => (
  <Flex
    gap={2}
    css={`
      flex-wrap: wrap;
      min-width: 0;
    `}
  >
    {texts.map(text => (
      <Pill key={text}>{text}</Pill>
    ))}
  </Flex>
);

const collapsedPillLimit = 10;

export const CollapsiblePills = ({ texts }: { texts: string[] }) => {
  const [expanded, setExpanded] = useState(false);
  const overflowNum = texts.length - collapsedPillLimit;

  const visible =
    expanded || overflowNum <= 0 ? texts : texts.slice(0, collapsedPillLimit);

  return (
    <Stack gap={2}>
      <Pills texts={visible} />
      {overflowNum > 0 && (
        <ExpandToggle onClick={() => setExpanded(e => !e)}>
          {expanded ? 'Show less' : `Show ${overflowNum} more`}
        </ExpandToggle>
      )}
    </Stack>
  );
};

const ExpandToggle = styled.button`
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  font-size: ${p => p.theme.fontSizes[1]}px;
  color: ${p => p.theme.colors.text.main};
  text-decoration: underline;
`;

const PillText = styled.span`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
  display: inline-block;
`;
