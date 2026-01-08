import styled, { CSSProp, useTheme } from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex } from 'design';

import { useAccessListManagementContext } from '../AccessListManagementContext';

export const MaxWidthBox = styled(Box)`
  width: 710px;
`;

export const StepButtons = ({
  disabled = false,
  disableNext = false,
  onNext,
  onPrev,
  nextBtnTxt,
  prevBtnTxt,
  cssStyle,
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
}) => {
  const { guideEditor: presetEditor } = useAccessListManagementContext();
  const { prevStep, nextStep } = presetEditor;
  const theme = useTheme();

  return (
    <Flex
      css={
        cssStyle
          ? cssStyle
          : `
      margin-top: ${theme.space[5]}px;
      margin-bottom: ${theme.space[8]}px;
      gap: ${theme.space[3]}px;
      width: 200px;
    `
      }
    >
      <ButtonPrimary
        onClick={onNext ? () => onNext() : () => nextStep()}
        disabled={disabled || disableNext}
        width="100%"
      >
        {nextBtnTxt ? nextBtnTxt : 'Next'}
      </ButtonPrimary>
      <ButtonSecondary
        width="100%"
        onClick={onPrev ? () => onPrev() : () => prevStep()}
        disabled={disabled}
      >
        {prevBtnTxt ? prevBtnTxt : 'Back'}
      </ButtonSecondary>
    </Flex>
  );
};
