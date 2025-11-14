import { MemoryRouter } from 'react-router';

import { act, fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { PluginProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { pluginMap } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/plugins';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  pluginsService,
  type CloudHostablePlugin,
} from 'e-teleport/services/plugins';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';
import { ContextProvider } from 'teleport';
import { ApiError } from 'teleport/services/api/parseError';
import {
  Integration,
  IntegrationAudience,
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';

import { AwsIcOidcIntegration } from './OidcIntegration';

const region = 'ca-central-1';
const instanceArn = 'arn:aws:sso:::instance/ssoins-xxxxx';
const integrationName = 'test-oidc';
const integrationRoleArn = `arn:aws:iam::123456789012:role/${integrationName}`;

const awsIdentityCenterPlugin = pluginMap[
  PluginConfigAwsIc.PluginName
] as CloudHostablePlugin;

const awsIcOidc: Integration = {
  resourceType: 'integration',
  name: integrationName,
  kind: IntegrationKind.AwsOidc,
  spec: {
    roleArn: integrationRoleArn,
    audience: IntegrationAudience.AwsIdentityCenter,
  },
  statusCode: IntegrationStatusCode.Running,
};

beforeEach(() => {
  jest.useFakeTimers();
  jest
    .spyOn(userEventService, 'captureIntegrationEnrollEvent')
    .mockImplementation();
  jest.spyOn(integrationService, 'fetchIntegrations').mockResolvedValue({
    items: [],
  });
});

afterEach(() => {
  jest.resetAllMocks();
  jest.useRealTimers();
});

describe('create integration and validation', () => {
  [
    {
      name: 'successful run',
      failCreateIntegration: false,
      resolveCreateIntegration: () => Promise.resolve(awsIcOidc),
      resolveIntegrationValidation: () => Promise.resolve({ message: 'ok' }),
      validationCount: 2,
    },
    {
      name: 'successful create integration but validation fails due to unknown reason',
      validationCount: 2,
    },
    {
      name: 'successful create integration but backend failed to fetch integration',
      resolveCreateIntegration: () => Promise.resolve(awsIcOidc),
      resolveIntegrationValidation: () =>
        Promise.reject(
          new ApiError({
            message: `integration "${integrationName}" doesn't exist`,
            response: { status: 404 } as Response,
          })
        ),
      validationCount: 4,
    },
  ].forEach(tc => {
    test(`${tc.name}`, async () => {
      const createFunc = jest
        .spyOn(integrationService, 'createIntegration')
        .mockImplementation(tc.resolveCreateIntegration);

      const validateFunc = jest
        .spyOn(pluginsService, 'validatePlugin')
        .mockReturnValueOnce(Promise.resolve({ message: 'ok' }))
        .mockImplementation(tc.resolveIntegrationValidation);

      const ctx = createTeleportContextE();
      render(
        <MemoryRouter>
          <ContextProvider ctx={ctx}>
            <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
              <AwsIcOidcIntegration />
            </PluginProvider>
          </ContextProvider>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(
          screen.getByText(/Step 1/i, { exact: false })
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByPlaceholderText(region), {
        target: { value: region },
      });
      fireEvent.change(screen.getByPlaceholderText(instanceArn), {
        target: { value: instanceArn },
      });
      fireEvent.change(screen.getByPlaceholderText(/Integration Name/i), {
        target: { value: integrationName },
      });
      fireEvent.click(
        screen.getByRole('button', {
          name: /Generate Script that configures AWS/i,
        })
      );

      await waitFor(() => {
        expect(
          screen.getByText(/Step 2: Run integration script/i, { exact: false })
        ).toBeInTheDocument();
      });
      expect(screen.getByText(/Step 3/i, { exact: false })).toBeInTheDocument();
      fireEvent.change(screen.getByPlaceholderText(integrationRoleArn), {
        target: { value: integrationRoleArn },
      });
      const saveButton = screen.getByRole('button', {
        name: /Save integration and proceed to the next step/i,
      });
      expect(saveButton).toBeEnabled();
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(
          screen.getByText(/OIDC Integration is created/i, { exact: false })
        ).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(createFunc).toHaveBeenCalled();
      });

      // If the validation request fails due to "integration doesn't exist" error,
      // we retry twice (total 3 attempts) with each attempt paused for 5 seconds
      // So the runAllTimers is called twice.
      await act(async () => jest.runAllTimers());
      await act(async () => jest.runAllTimers());
      await waitFor(() => {
        expect(validateFunc).toHaveBeenCalledTimes(tc.validationCount);
      });

      await waitFor(() => {
        expect(
          screen.getByRole('button', {
            name: /Validate credential and proceed to the next step/i,
          })
        ).toBeEnabled();
      });
    });
  });
});

test('permission validation', async () => {
  jest.spyOn(pluginsService, 'validatePlugin').mockImplementation(() =>
    Promise.reject(
      new ApiError({
        message: `You are missing the following permissions`,
        response: { status: 403 } as Response,
      })
    )
  );

  const ctx = createTeleportContextE();
  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcOidcIntegration />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(
      screen.getByText(/You are missing the following permissions/i, {
        exact: false,
      })
    ).toBeInTheDocument();
  });
});

test('permission validation error skipped on 501 error', async () => {
  jest.spyOn(pluginsService, 'validatePlugin').mockImplementation(() =>
    Promise.reject(
      new ApiError({
        message: `not implemented for AWS IC plugin`,
        response: { status: 501 } as Response,
      })
    )
  );

  const ctx = createTeleportContextE();
  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcOidcIntegration />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(screen.getByText(/Step 1/i, { exact: false })).toBeInTheDocument();
  });
});
