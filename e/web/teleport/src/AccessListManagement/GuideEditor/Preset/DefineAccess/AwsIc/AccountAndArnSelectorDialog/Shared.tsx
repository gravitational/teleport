import styled from 'styled-components';

import { Box, Flex } from 'design';

export const footerHeight = '60px';

export const OptionsContainer = styled(Box)`
  height: calc(100% - 90px);
  overflow: auto;
`;

export const OptionRow = styled(Flex)<{ disabled?: boolean }>`
  padding: ${p => p.theme.space[2]}px;
  gap: ${p => p.theme.space[2]}px;
  cursor: ${p => (p.disabled ? 'default' : 'pointer')};
  border-bottom: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  opacity: ${p => (p.disabled ? 0.6 : 1)};

  &:hover {
    background-color: ${p => p.theme.colors.interactive.tonal.primary[0]};
  }
`;
