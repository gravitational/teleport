import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import {
  accounts,
  DevNoteEnroll,
  groupsWithPermissionAssignment,
  integrationsResponse,
  permissionSets,
  users,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/shared/fixture';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { Plugin } from 'teleport/services/integrations';
import { User } from 'teleport/services/user';

import { renderPluginEnroll } from '../../StorybookHelper';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter',
};

export const Enroll = () => {
  const ctx = createTeleportContextE();
  ctx.userService.fetchUsers = () => Promise.resolve<User[]>(users);
  ctx.pluginsService.getAwsIcAccounts = () => Promise.resolve(accounts);
  ctx.pluginsService.getAwsIcGroupsWithPermissionAssignments = () =>
    Promise.resolve(groupsWithPermissionAssignment);
  ctx.pluginsService.getAwsIcPermissionSets = () =>
    Promise.resolve(permissionSets);
  ctx.pluginsService.validatePlugin = () => Promise.resolve({ message: 'ok' });
  ctx.pluginsService.createPlugin = () =>
    Promise.resolve<Plugin>({
      resourceType: 'plugin',
      name: 'aws-identity-center',
      kind: 'aws-identity-center',
      details: '',
      statusCode: 0,
    });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <DevNoteEnroll />
        {renderPluginEnroll(
          '',
          cfg.getIntegrationEnrollRoute('aws-identity-center'),
          ctx
        )}
      </ContextProvider>
    </MemoryRouter>
  );
};

Enroll.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(integrationsResponse)
      ),
      http.post(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(integrationsResponse)
      ),
    ],
  },
};
