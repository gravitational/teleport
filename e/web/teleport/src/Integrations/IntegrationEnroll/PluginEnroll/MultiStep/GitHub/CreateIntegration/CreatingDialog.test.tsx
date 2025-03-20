import { MemoryRouter } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';

import { ApiError } from 'teleport/services/api/parseError';
import {
  IntegrationGitHub,
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import ResourceService, {
  CreateOrOverwriteGitServer,
} from 'teleport/services/resources';

import { CreateIntegration } from './CreateIntegration';

afterEach(() => {
  jest.resetAllMocks();
});

const alreadyExistsError = new ApiError({
  message: '',
  response: { status: 409 } as Response,
});

const respGitHubIntegration: IntegrationGitHub = {
  resourceType: 'integration',
  name: 'created-integration-name',
  kind: IntegrationKind.GitHub,
  statusCode: IntegrationStatusCode.Running,
};

const createGitServerRequest: CreateOrOverwriteGitServer = {
  id: 'created-integration-name',
  subKind: IntegrationKind.GitHub,
  github: {
    integration: 'created-integration-name',
    organization: 'org-name',
  },
  overwrite: false,
};

test('with no errors, integration and server is both created with create handlers', async () => {
  const createIntegration = jest
    .spyOn(integrationService, 'createIntegration')
    .mockResolvedValue(respGitHubIntegration);

  const updateIntegration = jest
    .spyOn(integrationService, 'updateIntegration')
    .mockResolvedValue({} as any); // response doesn't matter

  const createServer = jest
    .spyOn(ResourceService.prototype, 'createOrOverwriteGitServer')
    .mockResolvedValue({} as any); // response doesn't matter

  await renderAndFillRequiredInputs();

  expect(screen.getByText(/created github integration/i)).toBeInTheDocument();
  expect(screen.getByText(/created git server/i)).toBeInTheDocument();

  expect(createIntegration).toHaveBeenCalledTimes(1);
  expect(updateIntegration).not.toHaveBeenCalled();
  expect(createServer).toHaveBeenCalledTimes(1);
  expect(createServer).toHaveBeenCalledWith(
    'localhost',
    createGitServerRequest
  );
});

test('integration exists error, calls integration update on overwrite next time', async () => {
  const createIntegration = jest
    .spyOn(integrationService, 'createIntegration')
    .mockRejectedValue(alreadyExistsError);

  const updateIntegration = jest
    .spyOn(integrationService, 'updateIntegration')
    .mockResolvedValue(respGitHubIntegration);

  const createServer = jest
    .spyOn(ResourceService.prototype, 'createOrOverwriteGitServer')
    .mockResolvedValue({} as any); // response doesn't matter

  await renderAndFillRequiredInputs();

  expect(
    screen.getByText(/github integration with the name/i)
  ).toBeInTheDocument();
  expect(screen.queryByText(/git server/i)).not.toBeInTheDocument();

  expect(createIntegration).toHaveBeenCalledTimes(1);
  expect(updateIntegration).not.toHaveBeenCalled();
  expect(createServer).not.toHaveBeenCalled();

  // resets function call counts from earlier back to 0.
  jest.clearAllMocks();

  await userEvent.click(screen.getByRole('button', { name: /overwrite/i }));

  await screen.findByText(/created git server/i);
  expect(screen.getByText(/overwrote github integration/i)).toBeInTheDocument();

  expect(createIntegration).not.toHaveBeenCalled();
  expect(updateIntegration).toHaveBeenCalledTimes(1);
  expect(createServer).toHaveBeenCalledWith(
    'localhost',
    createGitServerRequest
  );
});

test('server exists error, calls with overwrite flag set next time', async () => {
  const createIntegration = jest
    .spyOn(integrationService, 'createIntegration')
    .mockResolvedValue(respGitHubIntegration);

  const updateIntegration = jest
    .spyOn(integrationService, 'updateIntegration')
    .mockResolvedValue(respGitHubIntegration);

  const createServer = jest
    .spyOn(ResourceService.prototype, 'createOrOverwriteGitServer')
    .mockRejectedValueOnce(alreadyExistsError)
    .mockResolvedValue({} as any); // response doesn't matter

  await renderAndFillRequiredInputs();

  expect(screen.getByText(/created github integration/i)).toBeInTheDocument();
  expect(screen.getByText(/git server with the name/i)).toBeInTheDocument();

  expect(createIntegration).toHaveBeenCalledTimes(1);
  expect(updateIntegration).not.toHaveBeenCalled();
  expect(createServer).toHaveBeenCalledTimes(1);
  expect(createServer).toHaveBeenCalledWith(
    'localhost',
    createGitServerRequest
  );

  // resets function call counts from earlier back to 0.
  jest.clearAllMocks();

  await userEvent.click(screen.getByRole('button', { name: /overwrite/i }));
  await screen.findByText(/overwrote git server/i);
  expect(screen.getByText(/created github integration/i)).toBeInTheDocument();

  expect(createServer).toHaveBeenCalledTimes(1);
  expect(createServer).toHaveBeenCalledWith('localhost', {
    ...createGitServerRequest,
    overwrite: true,
  });

  // We already successfully created integration despite server creating failed,
  // so it shouldn't call again after its first attempt
  expect(createIntegration).not.toHaveBeenCalled();
  expect(updateIntegration).not.toHaveBeenCalled();
});

test('error not related to 409 conflict, still calls create handlers on retry', async () => {
  const createIntegration = jest
    .spyOn(integrationService, 'createIntegration')
    .mockRejectedValueOnce(new Error('some error'))
    .mockResolvedValue(respGitHubIntegration);

  const updateIntegration = jest
    .spyOn(integrationService, 'updateIntegration')
    .mockResolvedValue(respGitHubIntegration);

  let createServer = jest
    .spyOn(ResourceService.prototype, 'createOrOverwriteGitServer')
    .mockRejectedValueOnce(new Error('some error'));

  await renderAndFillRequiredInputs();

  // retry creating github integration
  expect(
    screen.getByText(/failed to create github integration/i)
  ).toBeInTheDocument();
  expect(screen.queryByText(/git server/i)).not.toBeInTheDocument();

  expect(createIntegration).toHaveBeenCalledTimes(1);
  expect(updateIntegration).not.toHaveBeenCalled();
  expect(createServer).not.toHaveBeenCalled();

  // resets function call counts from earlier back to 0.
  jest.clearAllMocks();

  await userEvent.click(screen.getByRole('button', { name: /retry/i }));
  await screen.findByText(/created github integration/i);
  expect(createIntegration).toHaveBeenCalledTimes(1);
  expect(updateIntegration).not.toHaveBeenCalled();

  // retry creating git server
  await screen.findByText(/Failed to create Git server:/i);

  // resets function call counts from earlier back to 0.
  jest.clearAllMocks();

  await userEvent.click(screen.getByRole('button', { name: /retry/i }));
  await screen.findByText(/created git server/i);

  expect(screen.getByText(/created github integration/i)).toBeInTheDocument();

  expect(createIntegration).not.toHaveBeenCalled();
  expect(updateIntegration).not.toHaveBeenCalled();
  expect(createServer).toHaveBeenCalledWith(
    'localhost',
    createGitServerRequest
  );
});

async function renderAndFillRequiredInputs() {
  render(
    <MemoryRouter>
      <CreateIntegration
        gitHubOrgName="org-name"
        onGitHubOrgNameChange={() => null}
        nextStep={() => null}
        prevStep={() => null}
        emitEvent={() => null}
      />
    </MemoryRouter>
  );

  await userEvent.click(
    screen.getByRole('button', { name: /start configuring/i })
  );

  expect(screen.getByRole('button', { name: /next/i })).toBeDisabled();

  await userEvent.type(screen.getByPlaceholderText(/1234567a/i), 'client-id');
  await userEvent.type(screen.getByPlaceholderText(/12a3b/i), 'client-secret');

  await userEvent.click(screen.getByRole('button', { name: /next/i }));

  expect(screen.getByTestId('dialogbox')).toBeInTheDocument();
}
