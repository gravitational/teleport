import styled from 'styled-components';

export const AccessMethodOptionContainer = styled.div<{ selected: boolean }>`
  display: flex;
  align-items: flex-start;
  gap: ${p => p.theme.space[3]}px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px
    ${p => p.theme.space[3]}px;
  border: 1px solid
    ${p =>
      p.selected
        ? p.theme.colors.brand
        : p.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${p => p.theme.radii[3]}px;
  cursor: pointer;
  flex: 1;

  &:hover {
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[1]};
  }
`;

export const RadioCircle = styled.div<{ selected: boolean }>`
  display: flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border: 1px solid
    ${p =>
      p.selected
        ? p.theme.colors.interactive.solid.primary.default
        : p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 50%;
  background: transparent;
`;

export const RadioDot = styled.div`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: ${p => p.theme.colors.interactive.solid.primary.default};
`;
