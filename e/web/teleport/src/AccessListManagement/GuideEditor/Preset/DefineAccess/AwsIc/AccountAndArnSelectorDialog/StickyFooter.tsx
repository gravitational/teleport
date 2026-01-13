import styled from 'styled-components';

import { Box, ButtonBorder, ButtonSecondary, Flex } from 'design';

import { footerHeight } from './Shared';

export function StickyFooter({
  onDialogClose,
  disabled,
  onAddSelection,
}: {
  onDialogClose(): void;
  disabled: boolean;
  onAddSelection(): void;
}) {
  return (
    <Footer>
      <Flex gap={3} alignItems="end" height="100%">
        <ButtonBorder
          intent="primary"
          width="150px"
          onClick={onAddSelection}
          disabled={disabled}
        >
          Add Selection
        </ButtonBorder>
        <ButtonSecondary
          onClick={onDialogClose}
          width="150px"
          disabled={disabled}
        >
          Cancel
        </ButtonSecondary>
      </Flex>
    </Footer>
  );
}

const Footer = styled(Box)`
  position: absolute;
  bottom: 0;
  background: ${props => props.theme.colors.levels.surface};
  height: ${footerHeight};
  width: 100%;
  right: 0;
`;
