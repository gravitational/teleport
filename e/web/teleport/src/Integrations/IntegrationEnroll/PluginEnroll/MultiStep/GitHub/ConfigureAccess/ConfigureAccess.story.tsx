import { delay, http, HttpResponse } from 'msw';
import { useState } from 'react';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import cfg from 'teleport/config';

import { ConfigureAccess } from './ConfigureAccess';

export default {
  title: 'TeleportE/Integrations/Enroll/GitHub/ConfigureAccess',
};

export const SkipOrCreateRole = () => {
  const [orgName, setOrgName] = useState('some-org');
  return (
    <MemoryRouter>
      <Info>
        Devs: the "finish" and "skip" button will render slightly different
        finish dialogs{' '}
      </Info>
      <ConfigureAccess
        gitHubOrgName={orgName}
        onGitHubOrgNameChange={setOrgName}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );
};
SkipOrCreateRole.beforeEach = ({ msw }) => {
  msw.use(http.post(cfg.api.role.create, () => HttpResponse.json({})));
};

export const CreateRoleLoading = () => {
  const [orgName, setOrgName] = useState('some-org');
  return (
    <MemoryRouter>
      <ConfigureAccess
        gitHubOrgName={orgName}
        onGitHubOrgNameChange={setOrgName}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );
};
CreateRoleLoading.beforeEach = ({ msw }) => {
  msw.use(http.post(cfg.api.role.create, () => delay('infinite')));
};

export const CreateRoleFailed = () => {
  const [orgName, setOrgName] = useState('some-org');
  return (
    <MemoryRouter>
      <ConfigureAccess
        gitHubOrgName={orgName}
        onGitHubOrgNameChange={setOrgName}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );
};
CreateRoleFailed.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.role.create, () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, error creating.' },
        },
        { status: 404 }
      )
    )
  );
};
