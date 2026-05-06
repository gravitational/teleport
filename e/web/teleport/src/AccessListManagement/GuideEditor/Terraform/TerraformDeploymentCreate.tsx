import { useRef, useState } from 'react';
import { useNavigate } from 'react-router';

import { Alert, Box, ButtonBorder, Flex, H1, H2, Stack, Text } from 'design';
import { useInterval } from 'shared/hooks';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { useCreateAccessList } from 'e-teleport/AccessListManagement/CreateAccessList/CreateAccessListContextProvider';
import { cancelPrompt } from 'e-teleport/AccessListManagement/CreateAccessList/types';
import {
  GuideContent,
  StepButtons,
} from 'e-teleport/AccessListManagement/GuideEditor/Shared';
import cfg from 'e-teleport/config';
import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { Prompt } from 'teleport/components/Router';
import {
  AccessListEvent,
  AccessListStepStatusEvent,
} from 'teleport/services/userEvent/accessListEvents';

import { SharedTerraformInstructions } from './SharedTerraformInstructions';

const POLLING_INTERVAL_MS = 3000; // 3 seconds

export function TerraformDeploymentCreate({ onPrev }: { onPrev(): void }) {
  const { guideEditor } = useAccessListManagementContext();
  const { terraform, isEditing, emitEvent } = guideEditor;

  const { newAccessListId } = useCreateAccessList();
  const navigate = useNavigate();
  const skipPromptRef = useRef(false);

  const [detectedAccessList, setDetectedAccessList] =
    useState<AccessList | null>(null);

  const [pollingDelay, setPollingDelay] = useState<number | null>(
    isEditing ? null : POLLING_INTERVAL_MS
  );

  useInterval(() => {
    accessManagementService
      .fetchAccessList(newAccessListId)
      .then(list => {
        setDetectedAccessList(list);
        setPollingDelay(null);
      })
      .catch(() => {
        // Access list not yet available,
        // ignore error and keep polling.
      });
  }, pollingDelay);

  function emitCompleteWithTerraformEvent() {
    emitEvent({
      event: AccessListEvent.Completed,
      stepStatus: AccessListStepStatusEvent.Success,
      preferredTerraform: true,
    });
  }

  function emitCompleteAndNavigate(route: string) {
    emitCompleteWithTerraformEvent();
    skipPromptRef.current = true;
    navigate(route);
  }

  function navigateToCreatedList(accessList: AccessList) {
    emitCompleteAndNavigate(cfg.getAccessListManagementRoute(accessList.id));
  }

  function handleDone() {
    if (!detectedAccessList && !window.confirm(cancelPrompt)) {
      return;
    }

    emitCompleteAndNavigate(cfg.getAccessListManagementRoute());
  }

  return (
    <>
      <Prompt
        when
        message={() => {
          if (skipPromptRef.current || detectedAccessList) {
            return true; // allow navigating away without prompt
          }
          return cancelPrompt;
        }}
      />
      <GuideContent withMaxWidth>
        <H1>Terraform Deployment</H1>
        <Box mb={3}>Best for teams managing infrastructure with Terraform.</Box>

        <H2 mb={3}>Run the following commands in your terminal:</H2>

        <Flex gap={4} flexDirection={'column'}>
          <SharedTerraformInstructions
            isEditing={isEditing}
            terraform={terraform}
          />

          <Stack mb={6}>
            <Text bold>7. Verify the list</Text>
            <Alert
              width="100%"
              kind={detectedAccessList ? 'success' : 'neutral'}
              primaryAction={
                detectedAccessList
                  ? {
                      content: 'View Created List',
                      onClick: () => navigateToCreatedList(detectedAccessList),
                    }
                  : undefined
              }
            >
              <Box>
                <Text bold>
                  {detectedAccessList
                    ? 'Access List Detected'
                    : 'Detecting your Access List'}
                </Text>
                {detectedAccessList && (
                  <Text typography="body2" bold={false}>
                    &quot;{detectedAccessList.title || detectedAccessList.id}
                    &quot; was successfully detected.
                  </Text>
                )}
                {!detectedAccessList && (
                  <Text typography="body2" bold={false}>
                    After you apply your Terraform configuration, Teleport will
                    auto detect your access list.
                  </Text>
                )}
              </Box>
            </Alert>
          </Stack>
        </Flex>
      </GuideContent>
      <StepButtons
        hideNextBtn
        onPrev={onPrev}
        customBtns={<ButtonBorder onClick={handleDone}>Done</ButtonBorder>}
      />
    </>
  );
}
