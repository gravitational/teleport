import styled from 'styled-components';

import { Flex, H1 } from 'design';

import { SectionDef, SectionId } from '../types';

export function TableOfContents({
  sections,
  activeId,
  onSelect,
}: {
  sections: SectionDef[];
  activeId: SectionId | undefined;
  onSelect: (id: SectionId) => void;
}) {
  const groups: Record<string, SectionDef[]> = {};
  for (const s of sections) {
    (groups[s.group] ??= []).push(s);
  }

  return (
    <Rail as="nav" aria-label="Beams Quickstart sections">
      <Header>Beams Quickstart</Header>
      {Object.entries(groups).map(([title, items]) => (
        <Group key={title}>
          <GroupTitle>{title}</GroupTitle>
          {items.map(s => (
            <Item
              key={s.id}
              as="a"
              href={`#${s.id}`}
              $active={s.id === activeId}
              aria-current={s.id === activeId ? 'true' : undefined}
              onClick={e => {
                if (e.metaKey || e.ctrlKey || e.shiftKey) return;
                e.preventDefault();
                onSelect(s.id);
              }}
            >
              {s.tocLabel ?? s.title}
            </Item>
          ))}
        </Group>
      ))}
    </Rail>
  );
}

const Rail = styled(Flex).attrs({ flexDirection: 'column', gap: 5 })`
  width: 240px;
  flex-shrink: 0;
  position: sticky;
  top: 0;
  align-self: flex-start;
  @media (max-width: ${p => p.theme.breakpoints.medium}) {
    display: none;
  }
`;

const Header = styled(H1)`
  font-weight: ${p => p.theme.fontWeights.regular};
  color: ${p => p.theme.colors.text.main};
  padding-top: ${p => p.theme.space[5]}px;
`;

const Group = styled(Flex).attrs({ flexDirection: 'column' })`
  border-left: 4px solid ${p => p.theme.colors.interactive.tonal.primary[2]};
`;

const GroupTitle = styled.div`
  font-size: ${p => p.theme.fontSizes[1]}px;
  line-height: 16px;
  letter-spacing: 0.15px;
  text-transform: uppercase;
  color: ${p => p.theme.colors.text.muted};
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[3]}px;
`;

const Item = styled.button<{ $active: boolean }>`
  ${p => p.theme.typography.subtitle2};
  display: block;
  width: 100%;
  text-align: left;
  font-family: inherit;
  text-decoration: none;
  background: ${p =>
    p.$active ? p.theme.colors.interactive.tonal.neutral[0] : 'transparent'};
  color: ${p =>
    p.$active ? p.theme.colors.text.main : p.theme.colors.text.slightlyMuted};
  border: none;
  border-radius: ${p => p.theme.radii[1]}px;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px
    ${p => p.theme.space[1]}px ${p => p.theme.space[3]}px;
  cursor: pointer;
  &:hover {
    background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
    color: ${p => p.theme.colors.text.main};
    text-decoration: none;
  }
  &:focus {
    outline: none;
  }
  &:focus-visible {
    outline: 2px solid ${p => p.theme.colors.brand};
    outline-offset: 2px;
  }
`;
