import styled from 'styled-components';

import { Flex } from 'design';

import { BeamsCard } from '../../components/BeamsCard';
import { renderBlock } from '../renderBlock';
import { CardDef, StepDef } from '../types';

const ICON_SIZE = 48;

export function StepCard({ card }: { card: CardDef }) {
  const Icon = card.icon;
  return (
    <Card>
      <IconColumn>
        <Icon size={ICON_SIZE} />
      </IconColumn>
      <Body>
        {card.steps?.map((step, i) => (
          <Step key={step.eyebrow ?? i} step={step} />
        ))}
        {card.trailing?.map(renderBlock)}
      </Body>
    </Card>
  );
}

function Step({ step }: { step: StepDef }) {
  return (
    <Group>
      {(step.eyebrow || step.title) && (
        <Header>
          {step.eyebrow && <Eyebrow>{step.eyebrow}</Eyebrow>}
          {step.title && <Title>{step.title}</Title>}
        </Header>
      )}
      {step.blocks?.map(renderBlock)}
    </Group>
  );
}

const Card = styled(BeamsCard)`
  display: flex;
  align-items: flex-start;
  gap: ${p => p.theme.space[5]}px;
  padding: ${p => p.theme.space[7]}px;
  /* Trailing pseudo-element mirrors the icon column so body content sits
     visually centered between two equal side columns. */
  &::after {
    content: '';
    flex-shrink: 0;
    width: ${ICON_SIZE}px;
  }
`;

const IconColumn = styled(Flex).attrs({
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: 0,
})`
  width: ${ICON_SIZE}px;
  height: ${ICON_SIZE}px;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const Body = styled(Flex).attrs({ flexDirection: 'column', gap: 7, flex: 1 })`
  min-width: 0;
`;

const Group = styled(Flex).attrs({ flexDirection: 'column', gap: 3 })``;

const Header = styled(Flex).attrs({ flexDirection: 'column', gap: 1 })`
  min-height: ${ICON_SIZE}px;
`;

const Eyebrow = styled.div`
  ${p => p.theme.typography.h4};
  font-weight: ${p => p.theme.fontWeights.regular};
  color: ${p => p.theme.colors.text.muted};
`;

const Title = styled.h3`
  ${p => p.theme.typography.body1};
  font-weight: ${p => p.theme.fontWeights.medium};
  color: ${p => p.theme.colors.text.main};
  margin: 0;
`;
