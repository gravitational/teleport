import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import cfg from 'teleport/config';
import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { GitHubRepoAccessDelete, Props } from './GitHubRepoAccessDelete';

export default {
  title: 'TeleportE/Integrations/Delete/GitHubRepoAccess',
};

const defaultProps: Props = {
  onClose: () => null,
  onRemove: () =>
    integrationService.deleteIntegration({
      name: 'github-org-name',
      clusterId: 'localhost',
    }),
  integration: {
    resourceType: 'integration',
    kind: IntegrationKind.GitHub,
    name: 'github-org-name',
    statusCode: IntegrationStatusCode.Running,
    spec: { organization: 'some-org' },
  },
};

const endpoint = cfg.getDeleteIntegrationUrlV2({
  clusterId: 'localhost',
  name: 'github-org-name',
});

export const Success = () => {
  return (
    <MemoryRouter>
      <Info>
        Devs: clicking on delete will not do anything on success state
      </Info>
      <GitHubRepoAccessDelete {...defaultProps} />
    </MemoryRouter>
  );
};
Success.beforeEach = ({ msw }) => {
  msw.use(http.delete(endpoint, () => HttpResponse.json({})));
};

export const IntegrationDeleteFailed = () => {
  return (
    <MemoryRouter>
      <GitHubRepoAccessDelete {...defaultProps} />
    </MemoryRouter>
  );
};
IntegrationDeleteFailed.beforeEach = ({ msw }) => {
  msw.use(
    http.delete(endpoint, () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, error deleting integration.' },
        },
        { status: 404 }
      )
    )
  );
};

export const Loading = () => {
  return (
    <MemoryRouter>
      <GitHubRepoAccessDelete {...defaultProps} />
    </MemoryRouter>
  );
};
Loading.beforeEach = ({ msw }) => {
  msw.use(http.delete(endpoint, () => delay('infinite')));
};
