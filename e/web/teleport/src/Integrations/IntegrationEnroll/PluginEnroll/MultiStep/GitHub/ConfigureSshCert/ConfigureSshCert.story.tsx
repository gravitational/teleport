import { delay, http, HttpResponse } from 'msw';
import { useState } from 'react';
import { MemoryRouter } from 'react-router';

import cfg from 'teleport/config';

import { ConfigureSshCert } from './ConfigureSshCert';

export default {
  title: 'TeleportE/Integrations/Enroll/GitHub/ConfigureSshCert',
};

export const Loaded = () => {
  const [orgName, setOrgName] = useState('some-org');
  return (
    <MemoryRouter>
      <ConfigureSshCert
        gitHubOrgName={orgName}
        onGitHubOrgNameChange={setOrgName}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );
};
Loaded.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.integrationCa.export, () =>
      HttpResponse.json({
        ssh: [{ publicKey: 'a public key', fingerprint: 'a thumb print' }],
      })
    )
  );
};

export const Loading = () => {
  const [orgName, setOrgName] = useState('some-org');
  return (
    <MemoryRouter>
      <ConfigureSshCert
        gitHubOrgName={orgName}
        onGitHubOrgNameChange={setOrgName}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );
};
Loading.beforeEach = ({ msw }) => {
  msw.use(http.get(cfg.api.integrationCa.export, () => delay('infinite')));
};

export const FetchFailed = () => {
  const [orgName, setOrgName] = useState('some-org');
  return (
    <MemoryRouter>
      <ConfigureSshCert
        gitHubOrgName={orgName}
        onGitHubOrgNameChange={setOrgName}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );
};
FetchFailed.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.integrationCa.export, () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, error creating.' },
        },
        { status: 404 }
      )
    )
  );
};
