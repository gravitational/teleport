import { H1, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  GuideContent,
  StepButtons,
} from 'e-teleport/AccessListManagement/GuideEditor/Shared';

import { useCreateAccessList } from '../../../CreateAccessListContextProvider';
import { SpecSection } from '../../../SpecSection';

export function BasicInformation() {
  const { reset: resetCreateContext } = useCreateAccessList();
  const { guideEditor } = useAccessListManagementContext();
  const { nextStep, currentStep, prevStep } = guideEditor;

  function handleNext(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    nextStep();
  }

  function handlePrev() {
    if (currentStep === 0) {
      resetCreateContext();
    }
    prevStep();
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          <GuideContent withMaxWidth mb={4}>
            <H1>Step {currentStep + 1}: Basic Information</H1>
            <Text mb={4}>
              Describe this access list and how frequently this access list
              should be audited.
            </Text>
            <SpecSection />
          </GuideContent>
          <StepButtons
            onNext={() => handleNext(validator)}
            onPrev={handlePrev}
          />
        </>
      )}
    </Validation>
  );
}
