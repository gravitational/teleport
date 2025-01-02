import { MemoryRouter } from 'react-router';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { idpMetadata } from 'e-teleport/SamlApplication/fixtures';
import { SamlApplicationProvider } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';

import { DownloadMetadata as DownloadMetadataComponent } from './DownloadMetadata';

export default {
  title: 'TeleportE/Discover/SAML Application',
};

export const DownloadMetadata = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  return (
    <Provider>
      <DownloadMetadataComponent />
    </Provider>
  );
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  const discoverCtx: DiscoverContextState = {
    ...props,
    currentStep: 0,
    onSelectResource: () => null,
    resourceSpec: undefined,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'app' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <SamlApplicationProvider>
          <DiscoverProvider mockCtx={discoverCtx}>
            {props.children}
          </DiscoverProvider>
        </SamlApplicationProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
