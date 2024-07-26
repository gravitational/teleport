import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { UserContext } from 'teleport/User/UserContext';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';
import {
  DiscoverProvider,
  DiscoverContextState,
  SamlMeta,
} from 'teleport/Discover/useDiscover';
import { DiscoverEventResource } from 'teleport/services/userEvent';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { Edit } from './Edit';

import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

export default {
  title: 'TeleportE/Discover/SAML Application/shared/SamlAppActions/Edit',
};

export const Default = () => {
  return (
    <Provider>
      <Edit
        {...props}
        attempt={{ status: 'success', data: samlMeta, statusText: '' }}
      />
    </Provider>
  );
};

export const Processing = () => {
  return (
    <Edit
      {...props}
      attempt={{ status: 'processing', data: null, statusText: '' }}
    />
  );
};

export const Error = () => {
  return (
    <Provider>
      <Edit
        {...props}
        attempt={{
          status: 'error',
          data: null,
          statusText: 'Error while fetching SAML resource',
          error: 'err',
        }}
      />
    </Provider>
  );
};

const samlMeta: SamlMeta = {
  samlGeneric: {
    kind: 'saml_idp_service_provider',
    metadata: {
      name: 'my_app',
      labels: {},
    },
    spec: {
      acs_url: 'https://example.com/saml/acs',
      attribute_mapping: [
        { name: 'role', name_format: 'uri', value: 'user.spec.role' },
      ],
      entity_descriptor: '<><>',
      entity_id: 'https://example.com/saml/eid',
      preset: 'unspecified',
      relay_state: '',
    },
    version: '',
  },
};

const idpMetadata = {
  entityID: 'https://tele.dev/enterprise/saml-idp/metadata',
  ssoURL: 'https://tele.dev/enterprise/saml-idp/sso',
  x509PEM:
    '-----BEGIN CERTIFICATE-----\nTUlJRGVqQ0NBbUtnQXdJQkFnSVFQc0x\nML1p5NGppVGVJYi81OXpzVE5tRQ==\n-----END CERTIFICATE-----\n',
};

const resourceSpec: ResourceSpec = {
  name: 'my_app',
  kind: 5,
  icon: 'application',
  keywords: 'saml',
  event: DiscoverEventResource.SamlApplication,
};

const props = {
  open: true,
  onClose: () => null,
  resourceSpec: resourceSpec,
  agentMeta: samlMeta,
  attempt: {},
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  ctx.idpService.updateSamlIdpServiceProvider = () => Promise.resolve(null);
  const discoverCtx: DiscoverContextState = {
    ...props,
    currentStep: 0,
    isUpdateFlow: true,
  };

  const updatePreferences = () => Promise.resolve();
  const getClusterPinnedResources = () => Promise.resolve([]);
  const updateClusterPinnedResources = () => Promise.resolve();
  const preferences: UserPreferences = makeDefaultUserPreferences();

  return (
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
  );
};
