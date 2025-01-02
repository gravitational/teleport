import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { idpMetadata } from 'e-teleport/SamlApplication/fixtures';
import { SamlApplicationProvider } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { ContextProvider } from 'teleport';

import { IdpMetadata as IdpMetadataComponent } from './IdpMetadata';

export default {
  title: 'TeleportE/SamlApplication/components/IdPMetadata',
};

export const IdpMetadata = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  ctx.idpService.getMetadataXml = () => Promise.resolve('<xml>test<xml>');
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SamlApplicationProvider>
          <IdpMetadataComponent />
        </SamlApplicationProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

export const IdpMetadataFileDownloadError = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  ctx.idpService.getMetadataXml = () => Promise.reject();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SamlApplicationProvider>
          <Info>
            Devs: Click Download Metadata File button to see file error label
          </Info>
          <IdpMetadataComponent />
        </SamlApplicationProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
