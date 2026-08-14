import styled from 'styled-components';

import { Flex, Text } from 'design';

export const OutlineBox = styled(Flex)<{ $hideBottomBorder?: boolean }>`
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 8px;
  padding: ${p => p.theme.space[3]}px;
  flex-direction: column;
  width: 620px;

  ${p => {
    if (p.$hideBottomBorder) {
      return {
        borderBottom: 'none',
      };
    }
  }}
`;

export const SmallHeader = styled(Text)`
  font-weight: ${p => p.theme.fontWeights.bold};
  font-size: ${p => p.theme.fontSizes[1]}px;
`;
