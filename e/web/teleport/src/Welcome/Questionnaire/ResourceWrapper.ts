import styled from 'styled-components';

import { Flex } from 'design';

export const ResourceWrapper = styled(Flex)<{
  isSelected?: boolean;
  invalid?: boolean;
}>`
  flex-direction: column;
  height: 100%;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: 12px;
  gap: 8px;
  border-radius: ${props => props.theme.radii[2]}px;

  border: ${({ isSelected, invalid, theme }) => {
    if (isSelected) {
      return `1px solid ${theme.colors.brand}`;
    }
    if (invalid) {
      return `1px solid ${theme.colors.error.main}`;
    }
    return `1px solid ${theme.colors.levels.elevated}`;
  }};

  &:hover {
    background-color: ${props => props.theme.colors.spotBackground[0]};
    box-shadow: ${({ theme }) => theme.boxShadow[2]};
  }

  &:focus-within {
    background-color: ${props => props.theme.colors.spotBackground[1]};
    border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
  }
`;
