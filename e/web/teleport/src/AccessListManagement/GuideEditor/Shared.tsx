import styled, { CSSProp, useTheme } from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import { Magnifier } from 'design/Icon';

import { FeatureBox } from 'teleport/components/Layout';

import { useAccessListManagementContext } from '../AccessListManagementContext';
import { stepButtonContainerHeight } from './const';

export const MaxWidthBox = styled(Box)`
  width: 710px;
`;

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
    <Box
      css={`
        position: fixed;
        bottom: 0;
        background-color: ${props => props.theme.colors.levels.sunken};
        border-top: 1px solid
          ${p => p.theme.colors.interactive.tonal.neutral[0]};

        left: 0px;
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
      height: ${stepButtonContainerHeight};
      margin-left: calc(var(--sidenav-width) + var(--guide-left-space));
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
    </Box>
  );
};

export const GuideContainer = styled(FeatureBox)`
  min-height: 100vh;
  --guide-left-space: ${props => props.theme.space[6]}px;
  padding-left: var(--guide-left-space);
`;
