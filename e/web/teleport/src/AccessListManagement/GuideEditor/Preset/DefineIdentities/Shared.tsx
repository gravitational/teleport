import styled from 'styled-components';

import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';

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
}: {
  nextBtnText?: string;
  onNext?(): void;
  onPrev?(): void;
}) => {
  const { guideEditor: presetEditor } = useAccessListManagementContext();
  const { prevStep, nextStep } = presetEditor;

  return (
    <Flex gap={3}>
      <ButtonPrimary
        width="100%"
        onClick={() => (onNext ? onNext() : nextStep())}
      >
        {nextBtnText ?? 'Next'}
      </ButtonPrimary>
      <ButtonSecondary
        width="100%"
        onClick={() => (onPrev ? onPrev() : prevStep())}
      >
        Back
      </ButtonSecondary>
    </Flex>
  );
};
