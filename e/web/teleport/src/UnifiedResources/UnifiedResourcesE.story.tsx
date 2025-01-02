import { MemoryRouter } from 'react-router';

import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  idpMetadata,
  mockSamlIdpServiceProvider,
} from 'e-teleport/SamlApplication/fixtures';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { ContentMinWidth } from 'teleport/Main/Main';
import { allAccessAcl } from 'teleport/mocks/contexts';
import { ResourcesResponse, UnifiedResource } from 'teleport/services/agents';
import type { Access } from 'teleport/services/user/types';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { UserContext } from 'teleport/User/UserContext';

import { UnifiedResourcesE } from './UnifiedResourcesE';

export default {
  title: 'TeleportE/UnifiedResources',
};

export const SamlAppEditAndDelete = () => {
  const customAcl = allAccessAcl;
  customAcl.samlIdpServiceProvider = samlAllAccess;

  return (
    <Provider {...customAcl}>
      <UnifiedResourcesE />
    </Provider>
  );
};

export const SamlAppEdit = () => {
  const customAcl = allAccessAcl;
  customAcl.samlIdpServiceProvider = { ...samlAllAccess, remove: false };
  return (
    <Provider {...customAcl}>
      <UnifiedResourcesE />
    </Provider>
  );
};

export const SamlAppDelete = () => {
  const customAcl = allAccessAcl;
  customAcl.samlIdpServiceProvider = { ...samlAllAccess, edit: false };

  return (
    <Provider {...customAcl}>
      <UnifiedResourcesE />
    </Provider>
  );
};

const Provider = props => {
  const discoverCtx: DiscoverContextState = {
    ...props,
    currentStep: 0,
    isUpdateFlow: true,
  };

  const updatePreferences = () => Promise.resolve();
  const getClusterPinnedResources = () => Promise.resolve([]);
  const updateClusterPinnedResources = () => Promise.resolve();
  const preferences: UserPreferences = makeDefaultUserPreferences();

  const ctx = createTeleportContextE({ customAcl: props.customAcl });
  ctx.resourceService.fetchUnifiedResources = () => Promise.resolve(resources);
  ctx.clusterService.fetchClusters = () => Promise.resolve([]);
  ctx.idpService.getSamlIdpServiceProvider = () =>
    Promise.resolve(mockSamlIdpServiceProvider);
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  ctx.idpService.updateSamlIdpServiceProvider = () => Promise.resolve(null);

  return (
    <ContentMinWidth>
      <MemoryRouter
        initialEntries={[
          { pathname: cfg.routes.discover, state: { entity: 'app' } },
        ]}
      >
        <UserContext.Provider
          value={{
            preferences,
            updatePreferences,
            getClusterPinnedResources,
            updateClusterPinnedResources,
          }}
        >
          <ContextProvider ctx={ctx}>
            <DiscoverProvider mockCtx={discoverCtx}>
              {props.children}
            </DiscoverProvider>
          </ContextProvider>
        </UserContext.Provider>
      </MemoryRouter>
    </ContentMinWidth>
  );
};

const samlAllAccess: Access = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

const resources: ResourcesResponse<UnifiedResource> = {
  agents: [
    {
      kind: 'app',
      name: 'saml_app',
      uri: '',
      publicAddr: '',
      description: 'SAML Application',
      awsConsole: false,
      labels: [],
      clusterId: 'one',
      fqdn: '',
      samlApp: true,
      samlAppSsoUrl: '',
      id: 'saml_app.teleport.com',
      launchUrl: '',
      awsRoles: [],
      userGroups: [],
    },
    {
      name: 'grafana',
      kind: 'app',
      uri: 'https://grafana',
      publicAddr: 'grafana.teleport.com',
      addrWithProtocol: 'https://grafana.teleport.com',
      labels: [],
      description: 'Grafana production',
      awsConsole: false,
      samlApp: false,
      awsRoles: [],
      clusterId: 'one',
      fqdn: 'grafana.com',
      id: 'grafana.teleport.com',
      launchUrl: '',
      userGroups: [],
    },
    {
      kind: 'node',
      id: 'some-id',
      clusterId: 'cluster-id',
      hostname: 'some-hostname',
      labels: [],
      addr: '',
      tunnel: false,
      subKind: 'teleport',
      sshLogins: [],
      awsMetadata: {
        accountId: 'aws-account-id',
        instanceId: 'instance-id',
        region: 'us-east-1',
        vpcId: 'instance-vpc-id',
        integration: 'integration-name',
        subnetId: 'subnet-id',
      },
    },
  ],
};
