import styled from 'styled-components';

import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { HoverTooltip } from 'design/Tooltip';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';

export const IdentityTabContainer = styled(Flex)`
  overflow: scroll;
  background-color: ${p => p.theme.colors.levels.elevated};
  border-radius: ${p => p.theme.space[2]}px;
  width: 390px;
  flex-direction: column;
  justify-content: space-between;
  height: 75vh;
  position: relative;
`;

export const IdentityStepButtons = ({
  nextBtnText,
  onNext,
  onPrev,
  nextBtnDisabled = false,
  nextBtnTooltipTxt,
}: {
  nextBtnText?: string;
  onNext?(): void;
  onPrev?(): void;
  nextBtnDisabled?: boolean;
  nextBtnTooltipTxt?: string;
}) => {
  const { guideEditor } = useAccessListManagementContext();
  const { prevStep, nextStep } = guideEditor;

  return (
    <StickyButtons>
      <Flex gap={3}>
        <HoverTooltip tipContent={nextBtnTooltipTxt}>
          <ButtonPrimary
            width="100%"
            onClick={() => (onNext ? onNext() : nextStep())}
            disabled={nextBtnDisabled}
          >
            {nextBtnText ?? 'Next'}
          </ButtonPrimary>
        </HoverTooltip>
        <ButtonSecondary
          width="100%"
          onClick={() => (onPrev ? onPrev() : prevStep())}
        >
          Back
        </ButtonSecondary>
      </Flex>
    </StickyButtons>
  );
};

const StickyButtons = styled(Box)`
  position: sticky;
  bottom: 0;
  background-color: ${props => props.theme.colors.levels.elevated};
  padding: 16px;
  padding-top: 16px;
  margin-top: 16px;
  border-top: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;
