import { ApiError } from 'teleport/services/api/parseError';

import { AttemptStatus } from './Status';

export default {
  title:
    'TeleportE/Integrations/Enroll/GitHub/CreateIntegration/CreatingDialog/Status',
};

const gitHubOrgName = 'some-github-organization-name';

const alreadyExistsError = new ApiError({
  message: '',
  response: { status: 409 } as Response,
});

export const Success = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{ status: 'success', data: null, statusText: '' }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{ status: 'success', data: null, statusText: '' }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
    </>
  );
};

export const CreatingProcessing = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{ status: 'processing', data: null, statusText: '' }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{ status: 'processing', data: null, statusText: '' }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
    </>
  );
};

export const OverwritingProcessing = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{ status: 'processing', data: null, statusText: '' }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{ status: 'processing', data: null, statusText: '' }}
      />
    </>
  );
};

export const OverwriteIntegration = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{ status: 'success', data: null, statusText: '' }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{ status: 'success', data: null, statusText: '' }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
    </>
  );
};

export const OverwriteGitServer = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{ status: 'success', data: null, statusText: '' }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{ status: 'success', data: null, statusText: '' }}
      />
    </>
  );
};

export const Overwrites = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{ status: 'success', data: null, statusText: '' }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{ status: 'success', data: null, statusText: '' }}
      />
    </>
  );
};

export const OverwriteError = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: new Error('some error'),
        }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: alreadyExistsError,
        }}
        updateAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: new Error('some error'),
        }}
      />
    </>
  );
};

export const CreateError = () => {
  return (
    <>
      <AttemptStatus
        kind="integration"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: new Error('some error'),
        }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
      <AttemptStatus
        kind="gitserver"
        resourceName={gitHubOrgName}
        createAttempt={{
          status: 'error',
          data: null,
          statusText: '',
          error: new Error('some error'),
        }}
        updateAttempt={{ status: '', data: null, statusText: '' }}
      />
    </>
  );
};
