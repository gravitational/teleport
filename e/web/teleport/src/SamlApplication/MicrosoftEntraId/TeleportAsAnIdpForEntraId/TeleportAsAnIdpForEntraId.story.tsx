import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  idpMetadata,
  resourceSpecSamlGeneric,
} from 'e-teleport/SamlApplication/fixtures';
import { SamlApplicationProvider } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';

import { TeleportAsAnIdpForEntraId as TeleportAsAnIdpForEntraIdComponent } from './TeleportAsAnIdpForEntraId';

export default {
  title: 'TeleportE/SamlApplication/MicrosoftEntraId',
};

export const TeleportAsAnIdpForEntraId = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  ctx.idpService.getMetadataXml = () => Promise.resolve('<xml>test<xml>');
  return (
    <RequiredDiscoverProviders
      agentMeta={{}}
      resourceSpec={resourceSpecSamlGeneric}
      teleportCtx={ctx}
    >
      <SamlApplicationProvider>
        <TeleportAsAnIdpForEntraIdComponent />
      </SamlApplicationProvider>
    </RequiredDiscoverProviders>
  );
};
