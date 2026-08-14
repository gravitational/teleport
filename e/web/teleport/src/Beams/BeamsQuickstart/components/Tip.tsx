import { ReactNode } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';

export function Tip({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <TipRow>
      <TipLabel>{label}:</TipLabel> {children}
    </TipRow>
  );
}

// TipList stacks multiple Tip rows with a tight gap so they read as a
// cluster, not separate paragraphs.
export const TipList = styled(Flex).attrs({
  flexDirection: 'column',
  gap: 1,
})``;

const TipRow = styled.div`
  ${p => p.theme.typography.subtitle2};
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const TipLabel = styled.span`
  font-weight: ${p => p.theme.fontWeights.bold};
`;
