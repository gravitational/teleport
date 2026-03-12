import { useState } from 'react';
import { generatePath, useNavigate } from 'react-router';

import { Alert } from 'design/Alert';
import { ButtonPrimary, ButtonSecondary, ButtonWarning } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Link from 'design/Link';
import Text from 'design/Text';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import {
  ExternalAuditStorage,
  IntegrationAwsOidc,
} from 'teleport/services/integrations';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';

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
  const navigate = useNavigate();
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

  async function handleContinuePreviousDraft() {
    setPreviousDraft(null);
    await continuePreviousDraft();
    return nextStep();
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
              <Alert kind="danger">{attemptDelete.statusText}</Alert>
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
              onClick={() => navigate(-1)}
            >
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}

      <Text mb="2">
        Teleport needs to set up an Athena workgroup, a Glue database and table,
        and S3 buckets for long-term and transient storage in your AWS account,
        as well as attach a policy with permissions necessary for the IAM role
        used with your existing AWS OIDC integration. All you need to do is
        generate and copy a script, then paste it into AWS CloudShell.
      </Text>

      {attemptGenerate.status === 'failed' && (
        <Alert kind="danger">{attemptGenerate.statusText}</Alert>
      )}
      {attempt.status === 'failed' && (
        <Alert kind="danger">{attempt.statusText}</Alert>
      )}
      {script && attempt.status !== 'processing' && (
        <>
          <TextSelectCopy mb="2" text={script} allowMultiline />
          <Text>
            Paste the command in{' '}
            <Link target="_blank" href={AWS_CLOUD_SHELL_LINK}>
              AWS CloudShell
            </Link>{' '}
          </Text>
        </>
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
  );
}

function getBootstrapScript(
  externalAuditStorage: ExternalAuditStorage | null,
  selectedAwsIntegration: IntegrationAwsOidc | null
): string {
  if (!externalAuditStorage || !selectedAwsIntegration) {
    return '';
  }
  const query = new URLSearchParams();
  const { awsAccountId, arnResourceName: iamRoleName } = splitAwsIamArn(
    selectedAwsIntegration.spec.roleArn
  );
  query.set('region', externalAuditStorage.region);
  query.set('role', iamRoleName || '');
  query.set('policy', externalAuditStorage.policyName);
  query.set('recordings', externalAuditStorage.sessionsRecordingsURI);
  query.set('events', externalAuditStorage.auditEventsLongTermURI);
  query.set('results', externalAuditStorage.athenaResultsURI);
  query.set('integration', externalAuditStorage.integrationName);
  query.set('workgroup', externalAuditStorage.athenaWorkgroup);
  query.set('db', externalAuditStorage.glueDatabase);
  query.set('table', externalAuditStorage.glueTable);
  query.set('awsAccountID', awsAccountId);
  const path = generatePath(cfg.api.externalAuditStorage.bootstrap, {
    clusterId: cfg.oss.proxyCluster,
  });
  return `bash -c "$(curl -fsSL  '${
    cfg.oss.baseUrl
  }${path}?${query.toString()}')"`;
}
