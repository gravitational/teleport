import { PropsWithChildren } from 'react';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  idpMetadata,
  resourceSpecSamlGeneric,
} from 'e-teleport/SamlApplication/fixtures';
import { SamlApplicationProvider } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';

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

const Provider: React.FC<PropsWithChildren> = props => {
  const ctx = createTeleportContextE();
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);

  return (
    <RequiredDiscoverProviders
      agentMeta={{}}
      resourceSpec={resourceSpecSamlGeneric}
      teleportCtx={ctx}
    >
      <SamlApplicationProvider>{props.children}</SamlApplicationProvider>
    </RequiredDiscoverProviders>
  );
};
