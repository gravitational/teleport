import { useEffect } from 'react';

import {
  AnimatedProgressBar,
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
  Flex,
  Text,
} from 'design';
import Dialog, { DialogContent } from 'design/DialogConfirmation';
import { useAsync } from 'shared/hooks/useAsync';
import { getErrMessage } from 'shared/utils/errorType';

import {
  IntegrationGitHub,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import ResourceService from 'teleport/services/resources';
import { IntegrationEnrollStatusCode } from 'teleport/services/userEvent/types';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { getIntegrationName } from '../getIntegrationName';
import { EmitEvent } from '../GitHub';
import { isAlreadyExistsError } from './error';
import { AttemptStatus } from './Status';

export function CreatingDialog({
  next,
  cancel,
  gitHubOrgName,
  clientId,
  clientSecret,
  emitEvent,
}: {
  next(): void;
  cancel(): void;
  gitHubOrgName: string;
  clientId: string;
  clientSecret: string;
  emitEvent(event: EmitEvent): void;
}) {
  const { clusterId } = useStickyClusterId();

  const integrationName = getIntegrationName(gitHubOrgName);

  const [createIntegrationAttempt, createIntegration] = useAsync(async () => {
    try {
      return await integrationService.createIntegration({
        name: integrationName,
        subKind: IntegrationKind.GitHub,
        oauth: {
          id: clientId,
          secret: clientSecret,
        },
        github: {
          organization: gitHubOrgName,
          oauthCallbackUrl: '/web/github/integration/callback',
        },
      });
    } catch (err) {
      emitEvent({
        status: {
          code: IntegrationEnrollStatusCode.Error,
          error: `Failed to create integration: ${getErrMessage(err)}`,
        },
      });
      throw err;
    }
  });

  const [updateIntegrationAttempt, updateIntegration] = useAsync(async () => {
    try {
      return await integrationService.updateIntegration(integrationName, {
        kind: IntegrationKind.GitHub,
        oauth: {
          id: clientId,
          secret: clientSecret,
        },
        github: {
          organization: gitHubOrgName,
          oauthCallbackUrl: '/web/github/integration/callback',
        },
      });
    } catch (err) {
      emitEvent({
        status: {
          code: IntegrationEnrollStatusCode.Error,
          error: `Failed to update integration: ${getErrMessage(err)}`,
        },
      });
      throw err;
    }
  });

  const [createGitServerAttempt, createGitServer] = useAsync(
    async (integration: IntegrationGitHub) => {
      const resourceService = new ResourceService();
      try {
        return await resourceService.createOrOverwriteGitServer(clusterId, {
          id: integration.name,
          subKind: 'github',
          overwrite: false,
          github: {
            organization: gitHubOrgName,
            integration: integration.name,
          },
        });
      } catch (err) {
        emitEvent({
          status: {
            code: IntegrationEnrollStatusCode.Error,
            error: `Failed to create git server: ${getErrMessage(err)}`,
          },
        });
        throw err;
      }
    }
  );

  const [updateGitServerAttempt, updateGitServer] = useAsync(
    async (integration: IntegrationGitHub) => {
      const resourceService = new ResourceService();
      try {
        return await resourceService.createOrOverwriteGitServer(clusterId, {
          id: integration.name,
          subKind: 'github',
          overwrite: true,
          github: {
            organization: gitHubOrgName,
            integration: integration.name,
          },
        });
      } catch (err) {
        emitEvent({
          status: {
            code: IntegrationEnrollStatusCode.Error,
            error: `Failed to update git server: ${getErrMessage(err)}`,
          },
        });
        throw err;
      }
    }
  );

  function onNext() {
    emitEvent({ status: { code: IntegrationEnrollStatusCode.Success } });
    next();
  }

  const success =
    (createGitServerAttempt.status === 'success' ||
      updateGitServerAttempt.status === 'success') &&
    (createIntegrationAttempt.status === 'success' ||
      updateIntegrationAttempt.status === 'success');

  const processing =
    createIntegrationAttempt.status === 'processing' ||
    createIntegrationAttempt.status === '' ||
    createGitServerAttempt.status === 'processing' ||
    updateGitServerAttempt.status === 'processing' ||
    updateIntegrationAttempt.status === 'processing';

  async function onCreateResources() {
    let createdInt = createIntegrationAttempt.data;
    if (createIntegrationAttempt.status !== 'success') {
      const [resp, err] = await createIntegration();
      if (err) {
        return;
      }
      createdInt = resp;
    }

    await createGitServer(createdInt);
  }

  useEffect(() => {
    onCreateResources();
  }, []);

  async function onUpdateIntegration() {
    let createdInt = createIntegrationAttempt.data;
    if (updateIntegrationAttempt.status !== 'success') {
      const [resp, err] = await updateIntegration();
      if (err) {
        return;
      }
      createdInt = resp;
    }

    await createGitServer(createdInt);
  }

  async function onUpdateGitServer() {
    let createdInt = createIntegrationAttempt.data;
    if (updateIntegrationAttempt.status === 'success') {
      createdInt = updateIntegrationAttempt.data;
    }

    await updateGitServer(createdInt);
  }

  function renderButton() {
    if (success) {
      return (
        <ButtonPrimary width="100%" onClick={onNext}>
          Next
        </ButtonPrimary>
      );
    }

    if (processing) {
      return (
        <ButtonPrimary width="100%" disabled={true}>
          Next
        </ButtonPrimary>
      );
    }

    let PrimaryButton;

    if (updateIntegrationAttempt.status === 'error') {
      PrimaryButton = (
        <ButtonWarning width="100%" onClick={onUpdateIntegration}>
          Retry
        </ButtonWarning>
      );
    } else if (updateGitServerAttempt.status === 'error') {
      PrimaryButton = (
        <ButtonWarning width="100%" onClick={onUpdateGitServer}>
          Retry
        </ButtonWarning>
      );
    } else if (createGitServerAttempt.status === 'error') {
      PrimaryButton = (
        <ButtonPrimary onClick={onCreateResources}>Retry</ButtonPrimary>
      );

      if (isAlreadyExistsError(createGitServerAttempt)) {
        PrimaryButton = (
          <ButtonWarning width="100%" onClick={onUpdateGitServer}>
            Overwrite
          </ButtonWarning>
        );
      }
    } else if (createIntegrationAttempt.status === 'error') {
      PrimaryButton = (
        <ButtonPrimary onClick={onCreateResources}>Retry</ButtonPrimary>
      );

      if (isAlreadyExistsError(createIntegrationAttempt)) {
        PrimaryButton = (
          <ButtonWarning width="100%" onClick={onUpdateIntegration}>
            Overwrite
          </ButtonWarning>
        );
      }
    }

    return (
      <Flex gap={3}>
        {PrimaryButton}
        <ButtonSecondary onClick={cancel}>Cancel</ButtonSecondary>
      </Flex>
    );
  }

  return (
    <Dialog open={true}>
      <DialogContent
        width="460px"
        mb={0}
        textAlign="center"
        alignItems="center"
      >
        <Text bold caps mb={3}>
          Creating Required Resources
        </Text>
        {processing && <AnimatedProgressBar mb={3} />}
        <Flex flexDirection="column" alignItems="center" minHeight="90px">
          <AttemptStatus
            kind="integration"
            resourceName={integrationName}
            createAttempt={createIntegrationAttempt}
            updateAttempt={updateIntegrationAttempt}
          />
          <AttemptStatus
            kind="gitserver"
            resourceName={integrationName}
            createAttempt={createGitServerAttempt}
            updateAttempt={updateGitServerAttempt}
          />
        </Flex>
        {renderButton()}
      </DialogContent>
    </Dialog>
  );
}
