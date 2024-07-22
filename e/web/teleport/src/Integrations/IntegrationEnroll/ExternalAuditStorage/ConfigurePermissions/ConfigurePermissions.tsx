import React, { useState } from 'react';
import { useHistory } from 'react-router';
import { generatePath } from 'react-router';
import { HeaderSubtitle, Header } from 'teleport/Discover/Shared';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import useAttempt from 'shared/hooks/useAttemptNext';
import Text from 'design/Text';
import Link from 'design/Link';
import { ButtonPrimary, ButtonSecondary, ButtonWarning } from 'design/Button';

import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import {
  ExternalAuditStorage,
  Integration,
} from 'teleport/services/integrations';

import Alert from 'design/Alert';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';

import { useExternalAuditStorage } from '../useExternalAuditStorage';

const AWS_CLOUD_SHELL_LINK = 'https://console.aws.amazon.com/cloudshell/home';

export function ConfigurePermissions() {
  const { externalAuditStorageService } = useTeleportE();
  const [previousDraft, setPreviousDraft] =
    useState<ExternalAuditStorage>(null);

  const {
    nextStep,
    selectedAwsIntegration,
    draft,
    createDraft,
    continuePreviousDraft,
    attempt,
  } = useExternalAuditStorage();
  const history = useHistory();
  const script = getBootstrapScript(draft, selectedAwsIntegration);
  const { run: runGenerate, attempt: attemptGenerate } = useAttempt('');
  const { run: runDelete, attempt: attemptDelete } = useAttempt('');

  function handleGenerate() {
    return runGenerate(() =>
      externalAuditStorageService.getDraft().then(result => {
        // if a draft already exists, warn users
        if (result) {
          setPreviousDraft(result);
          return;
        }
        // if there is no existing draft, create a draft and enable test step
        runGenerate(() => createDraft().then(() => nextStep()));
      })
    );
  }

  function deleteDraft() {
    return runDelete(() =>
      externalAuditStorageService.deleteDraft().then(() => {
        setPreviousDraft(null);
        createDraft().then(() => nextStep());
      })
    );
  }

  function handleContinuePreviousDraft() {
    setPreviousDraft(null);
    return continuePreviousDraft().then(() => nextStep());
  }

  return (
    <>
      {previousDraft && (
        <Dialog open={true}>
          <DialogHeader>
            <DialogTitle>Draft in progress</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attemptDelete.status === 'failed' && (
              <Alert kind="danger" children={attemptDelete.statusText} />
            )}
            <Text>
              There is an ongoing External Audit Storage integration with{' '}
              <b>{previousDraft.integrationName}</b> currently in progress.
            </Text>
            <Text>Continuing it may impact the ongoing configuration.</Text>
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="2"
              disabled={attemptDelete.status === 'processing'}
              onClick={handleContinuePreviousDraft}
            >
              Continue draft
            </ButtonPrimary>
            <ButtonWarning
              mr="2"
              onClick={deleteDraft}
              disabled={attemptDelete.status === 'processing'}
            >
              Delete draft
            </ButtonWarning>
            <ButtonSecondary
              mr="2"
              disabled={attemptDelete.status === 'processing'}
              onClick={history.goBack}
            >
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}

      <Header>Configure Permissions</Header>
      <HeaderSubtitle>
        Teleport needs to create S3 buckets, a Glue database, and an Athena
        workgroup in your AWS account. This step will also attach a policy with
        the necessary permissions to the IAM role used with your existing AWS
        OIDC integration.
      </HeaderSubtitle>

      <>
        <Text bold>Set up the infrastructure</Text>
        <Text mb="4" mt="1">
          Open{' '}
          <Link target="_blank" href={AWS_CLOUD_SHELL_LINK}>
            Amazon CloudShell
          </Link>{' '}
          copy/paste the following command to set up an Athena workgroup, a Glue
          database and table, and S3 buckets for long-term and transient
          storage.
        </Text>
        {attemptGenerate.status === 'failed' && (
          <Alert kind="danger" children={attemptGenerate.statusText} />
        )}
        {attempt.status === 'failed' && (
          <Alert kind="danger" children={attempt.statusText} />
        )}
        {script && attempt.status !== 'processing' && (
          <TextSelectCopy mb="4" text={script} allowMultiline />
        )}
        {attempt.status === 'processing' && <Text>Loading...</Text>}
        {!script && (
          <ButtonPrimary
            onClick={handleGenerate}
            disabled={
              attemptGenerate.status === 'processing' ||
              attempt.status === 'processing'
            }
          >
            Generate Script
          </ButtonPrimary>
        )}
      </>
    </>
  );
}

function getBootstrapScript(
  externalAuditStorage: ExternalAuditStorage | null,
  selectedAwsIntegration: Integration | null
): string {
  if (!externalAuditStorage || !selectedAwsIntegration) {
    return '';
  }
  const query = new URLSearchParams();
  query.set('region', externalAuditStorage.region);
  query.set(
    'role',
    selectedAwsIntegration.spec.roleArn.split(':role/')[1] || ''
  );
  query.set('policy', externalAuditStorage.policyName);
  query.set('recordings', externalAuditStorage.sessionsRecordingsURI);
  query.set('events', externalAuditStorage.auditEventsLongTermURI);
  query.set('results', externalAuditStorage.athenaResultsURI);
  query.set('integration', externalAuditStorage.integrationName);
  query.set('workgroup', externalAuditStorage.athenaWorkgroup);
  query.set('db', externalAuditStorage.glueDatabase);
  query.set('table', externalAuditStorage.glueTable);
  const path = generatePath(cfg.api.externalAuditStorage.bootstrap, {
    clusterId: cfg.oss.proxyCluster,
  });
  return `bash -c "$(curl -fsSL  '${
    cfg.oss.baseUrl
  }${path}?${query.toString()}')"`;
}
