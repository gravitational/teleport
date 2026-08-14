import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import ecfg from 'e-teleport/config';
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
  ctx.userService.fetchUsersV2 = () =>
    Promise.resolve({ items: users as User[], startKey: '' });
  ctx.pluginsService.getAwsIcAccounts = () => Promise.resolve(accounts);
  ctx.pluginsService.getAwsIcGroupsWithPermissionAssignments = () =>
    Promise.resolve(groupsWithPermissionAssignment);
  ctx.pluginsService.getAwsIcPermissionSets = () =>
    Promise.resolve(permissionSets);
  ctx.pluginsService.validatePlugin = () => Promise.resolve({ message: 'ok' });
  ctx.pluginsService.createStaticAuthPlugin = () =>
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

Enroll.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponse)
    ),
    http.post(cfg.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponse)
    )
  );
};

export const MissingPermissions = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        {renderPluginEnroll(
          '',
          cfg.getIntegrationEnrollRoute('aws-identity-center'),
          ctx
        )}
      </ContextProvider>
    </MemoryRouter>
  );
};

MissingPermissions.beforeEach = ({ msw }) => {
  msw.use(
    http.post(ecfg.getPluginValidateUrl(), () =>
      HttpResponse.json(
        {
          error: {
            message: `You are missing the following permissions to complete this plugin installation:\n- Verb create on resource kind integration\n- Verb create on resource kind saml_idp_service_provider\n- Version 8 role allowing "app_labels" matching label "teleport.dev/origin : aws-identity-center"`,
            response: { status: 401 } as Response,
          },
        },
        { status: 401 }
      )
    )
  );
};
