import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import cfg from 'teleport/config';

import { CreatingDialog } from './CreatingDialog';

export default {
  title:
    'TeleportE/Integrations/Enroll/GitHub/CreateIntegration/CreatingDialog',
};

const defaultProps = {
  gitHubOrgName: 'some-org',
  next: () => null,
  cancel: () => null,
  clientId: '',
  clientSecret: '',
  emitEvent: () => null,
};

export const Success = () => {
  return (
    <MemoryRouter>
      <CreatingDialog {...defaultProps} />
    </MemoryRouter>
  );
};
Success.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.integrationsPath, () => HttpResponse.json({})),
    http.put(cfg.api.gitServer.createOrOverwrite, () => HttpResponse.json({}))
  );
};

export const LoadingCreateIntegration = () => {
  return (
    <MemoryRouter>
      <CreatingDialog {...defaultProps} />
    </MemoryRouter>
  );
};
LoadingCreateIntegration.beforeEach = ({ msw }) => {
  msw.use(http.post(cfg.api.integrationsPath, () => delay('infinite')));
};

export const LoadingCreateServer = () => {
  return (
    <MemoryRouter>
      <CreatingDialog {...defaultProps} />
    </MemoryRouter>
  );
};
LoadingCreateServer.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.integrationsPath, () => HttpResponse.json({})),
    http.put(cfg.api.gitServer.createOrOverwrite, () => delay('infinite'))
  );
};

export const FailedCreateIntegration = () => {
  return (
    <MemoryRouter>
      <CreatingDialog {...defaultProps} />
    </MemoryRouter>
  );
};
FailedCreateIntegration.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.integrationsPath, () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, error creating integration.' },
        },
        { status: 404 }
      )
    )
  );
};

export const FailedCreateServer = () => {
  return (
    <MemoryRouter>
      <CreatingDialog {...defaultProps} />
    </MemoryRouter>
  );
};
FailedCreateServer.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.integrationsPath, () => HttpResponse.json({})),
    http.put(cfg.api.gitServer.createOrOverwrite, () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, error creating server.' },
        },
        { status: 404 }
      )
    )
  );
};

export const IntegrationAlreadyExists = () => {
  return (
    <MemoryRouter>
      <CreatingDialog {...defaultProps} />
    </MemoryRouter>
  );
};
IntegrationAlreadyExists.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.integrationsPath, () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, error creating integration.' },
        },
        { status: 409 }
      )
    )
  );
};

export const ServerAlreadyExists = () => {
  return (
    <MemoryRouter>
      <CreatingDialog {...defaultProps} />
    </MemoryRouter>
  );
};
ServerAlreadyExists.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.integrationsPath, () => HttpResponse.json({})),
    http.put(cfg.api.gitServer.createOrOverwrite, () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, error creating server.' },
        },
        { status: 409 }
      )
    )
  );
};
