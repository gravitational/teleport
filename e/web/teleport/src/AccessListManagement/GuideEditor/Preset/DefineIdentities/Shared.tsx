import styled from 'styled-components';

import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { HoverTooltip } from 'design/Tooltip';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';

export const IdentityTabContainer = styled(Flex)`
  overflow: scroll;
  background-color: ${p => p.theme.colors.levels.elevated};
  border-radius: ${p => p.theme.space[2]}px;
  width: 390px;
  padding: ${p => p.theme.space[2]}px;
  flex-direction: column;
  justify-content: space-between;
  height: 75vh;
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
  );
};
