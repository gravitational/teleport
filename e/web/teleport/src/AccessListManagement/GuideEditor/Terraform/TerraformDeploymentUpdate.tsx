import { ButtonBorder, Flex, H1, H2 } from 'design';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  GuideContent,
  StepButtons,
} from 'e-teleport/AccessListManagement/GuideEditor/Shared';

import { SharedTerraformInstructions } from './SharedTerraformInstructions';

export function TerraformDeploymentUpdate({
  onPrev,
  onClose,
}: {
  onPrev(): void;
  onClose(): void;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { terraform, isEditing } = guideEditor;

  return (
    <>
      <GuideContent withMaxWidth height="100vh">
        <H1>Update Terraform Deployment</H1>
        <H2 mb={3}>Run the following commands in your terminal:</H2>

        <Flex gap={4} flexDirection={'column'}>
          <SharedTerraformInstructions
            isEditing={isEditing}
            terraform={terraform}
          />
        </Flex>
      </GuideContent>
      <StepButtons
        hideNextBtn
        onPrev={onPrev}
        customBtns={<ButtonBorder onClick={onClose}>Done</ButtonBorder>}
      />
    </>
  );
}
