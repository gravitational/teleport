import { http, HttpResponse } from 'msw';
import { useState } from 'react';
import { MemoryRouter } from 'react-router';

import cfg from 'teleport/config';

import { CreateIntegration } from './CreateIntegration';

export default {
  title: 'TeleportE/Integrations/Enroll/GitHub/CreateIntegration',
};

export const Success = () => {
  const [orgName, setOrgName] = useState('some-org');
  return (
    <MemoryRouter>
      <CreateIntegration
        gitHubOrgName={orgName}
        onGitHubOrgNameChange={setOrgName}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );
};
Success.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.integrationsPath, () => HttpResponse.json({})),
    http.put(cfg.api.gitServer.createOrOverwrite, () => HttpResponse.json({}))
  );
};
