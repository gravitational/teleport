import styled from 'styled-components';

import { Text, Box } from 'design';

export type FeatureProps = {
  isSliding: boolean;
  active: boolean;
  onClick(): void;
};

export const Title = styled(Text)`
  font-weight: bold;
`;

export const Description = styled(Text)`
  font-size: ${p => p.theme.fontSizes[1]};
`;

export const Feature = styled(Box)<{ $isSliding?: boolean; $active?: boolean }>`
height: var(--feature-height);

  line-height: 20px;
  padding: ${p => p.theme.space[3]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  cursor: pointer;
  width: var(--feature-width);

  background-color: ${p =>
    !p.$isSliding && p.$active
      ? p => p.theme.colors.interactive.tonal.primary[0].background
      : 'inherit'};

  ${Title} {
    color: ${p => {
      // TODO(lisa): add another color theme to `interactive` object
      // instead of referring to the same color for button pallette.
      if (p.$isSliding && p.$active) {
        return p.theme.colors.buttons.primary.default;
      }
      return p.$active ? p.theme.colors.buttons.primary.default : 'inherit';
    }}};
    transition: color 0.2s ease-in 0s;
  }

  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
  }

  &:hover ${Title} {
    color: ${p => p.theme.colors.text.main};
  }
`;
