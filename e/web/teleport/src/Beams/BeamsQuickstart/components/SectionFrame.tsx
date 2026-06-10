import { ReactNode } from 'react';
import styled from 'styled-components';

import { Flex, H2 } from 'design';

import { SectionId } from '../types';

export function SectionFrame({
  id,
  title,
  active,
  children,
}: {
  id: SectionId;
  title: string;
  active: boolean;
  children: ReactNode;
}) {
  return (
    <Section id={id} data-active={active || undefined}>
      <SectionTitle>{title}</SectionTitle>
      <Cards>{children}</Cards>
    </Section>
  );
}

const Section = styled.section`
  &[data-active] > h2 {
    color: ${p => p.theme.colors.text.main};
    background: color-mix(
      in srgb,
      ${p => p.theme.colors.levels.sunken} 55%,
      transparent
    );
    backdrop-filter: blur(30px);
    mask-image: linear-gradient(to bottom, black 75%, transparent 100%);
  }
`;

const SectionTitle = styled(H2)`
  line-height: ${p => p.theme.typography.h1.lineHeight};
  font-weight: ${p => p.theme.fontWeights.regular};
  position: sticky;
  top: 0;
  z-index: 1;
  background: ${p => p.theme.colors.levels.sunken};
  color: ${p => p.theme.colors.text.slightlyMuted};
  padding: ${p => p.theme.space[5]}px 0;
  backdrop-filter: blur(20px);
  transition: color 150ms ease;
`;

const Cards = styled(Flex).attrs({ flexDirection: 'column', gap: 3 })``;
