import styled, { CSSProp, useTheme } from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import { Magnifier } from 'design/Icon';

import { useAccessListManagementContext } from '../AccessListManagementContext';

export const defaultSidePanelWidth = 365;

export const StyledUl = styled.ul`
  margin: 0;
  padding-left: ${p => p.theme.space[3]}px;
`;

export function NoResultsFound() {
  return (
    <Flex gap={2}>
      <Magnifier size="small" />
      <Text>No results found</Text>
    </Flex>
  );
}

export const StepButtons = ({
  disabled = false,
  disableNext = false,
  onNext,
  onPrev,
  nextBtnTxt,
  prevBtnTxt,
  cssStyle,
  hidePrevBtn = false,
  hideNextBtn = false,
  customBtns,
}: {
  /**
   * If true, disables all buttons
   */
  disabled?: boolean;
  /**
   * Disables next button
   */
  disableNext?: boolean;
  onNext?(): void;
  onPrev?(): void;
  nextBtnTxt?: string;
  prevBtnTxt?: string;
  cssStyle?: CSSProp;
  hidePrevBtn?: boolean;
  hideNextBtn?: boolean;
  customBtns?: React.ReactNode;
}) => {
  const { guideEditor: presetEditor } = useAccessListManagementContext();
  const { prevStep, nextStep } = presetEditor;
  const theme = useTheme();

  return (
    <Box
      px={6}
      py={3}
      css={`
        z-index: 100;
        position: sticky;
        bottom: 0;
        background-color: ${props => props.theme.colors.levels.sunken};
        border-top: 1px solid
          ${p => p.theme.colors.interactive.tonal.neutral[0]};
        left: 0;
        right: 0;
      `}
    >
      <Flex
        justifyContent="center"
        alignItems="center"
        css={
          cssStyle
            ? cssStyle
            : `
      gap: ${theme.space[3]}px;
      width: 200px;
    `
        }
      >
        {!hideNextBtn && (
          <ButtonPrimary
            onClick={onNext ? () => onNext() : () => nextStep()}
            disabled={disabled || disableNext}
            width="100%"
          >
            {nextBtnTxt ? nextBtnTxt : 'Next'}
          </ButtonPrimary>
        )}
        {!hidePrevBtn && (
          <ButtonSecondary
            width="100%"
            onClick={onPrev ? () => onPrev() : () => prevStep()}
            disabled={disabled}
          >
            {prevBtnTxt ? prevBtnTxt : 'Back'}
          </ButtonSecondary>
        )}
        <>{customBtns}</>
      </Flex>
    </Box>
  );
};

export const GuideContainer = styled(Flex)`
  height: 100%;
  flex-direction: column;
  justify-content: space-between;
`;

export const GuideContent = styled(Box)<{ withMaxWidth?: boolean }>`
  padding-left: ${props => props.theme.space[6]}px;
  padding-right: ${props => props.theme.space[6]}px;
  padding-top: ${props => props.theme.space[3]}px;

  width: ${p => (p.withMaxWidth ? '710px' : 'auto')};
`;
