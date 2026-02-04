import { Alert, Box, Flex, H2 } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  GuideContent,
  StepButtons,
} from 'e-teleport/AccessListManagement/GuideEditor/Shared';

import { useCreateAccessList } from '../../CreateAccessListContextProvider';
import { MembersSection } from '../../MemberSection';
import { OwnersSection } from '../../OwnerSection';
import { SpecSection } from '../../SpecSection';

/**
 * AccessListForm is a single step flow rendering all fields allowing
 * user to manually enter all fields however they like.
 */
export function AccessListForm() {
  const {
    onCreate,
    createAttempt,
    reset: resetCreateContext,
  } = useCreateAccessList();
  const { guideEditor } = useAccessListManagementContext();
  const { prevStep, currentStep, nextStep } = guideEditor;

  function handlePrev() {
    if (currentStep === 0) {
      resetCreateContext();
    }
    prevStep();
  }

  async function handleFinish(validator: Validator) {
    const created = await onCreate(validator);
    if (created) {
      nextStep();
    }
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          <GuideContent withMaxWidth mb={5}>
            <H2 mb={3}>Basic Information</H2>
            <Flex flexDirection="column" gap={4}>
              <Box mb={6}>
                <SpecSection />
              </Box>
              <Box>
                <OwnersSection />
              </Box>
              <Box>
                <MembersSection />
              </Box>
            </Flex>
            {createAttempt.status === 'failed' && (
              <Alert>{createAttempt.statusText}</Alert>
            )}
          </GuideContent>
          <StepButtons
            onNext={() => handleFinish(validator)}
            onPrev={handlePrev}
            disabled={createAttempt.status === 'processing'}
          />
        </>
      )}
    </Validation>
  );
}
