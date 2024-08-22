import React from 'react';
import { MemoryRouter } from 'react-router';
import { Info } from 'design/Alert';
import { ContextProvider } from 'teleport';

import { idpMetadata } from 'e-teleport/Discover/SamlApplication/shared/fixtures';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { ConfigureServiceProvider } from './DownloadMetadata';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';

export default {
  title: 'TeleportE/Discover/SAML Application',
};

export const DownloadMetadata = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getMetadataXml = () => Promise.resolve('<xml>test<xml>');
  const samlIdPMetadata: SAMLIdPMetadataResponse = idpMetadata;

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ConfigureServiceProvider samlIdPMetadata={samlIdPMetadata} />
      </ContextProvider>
    </MemoryRouter>
  );
};

export const DownloadMetadataFileError = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getMetadataXml = () => Promise.reject();
  const samlIdPMetadata: SAMLIdPMetadataResponse = idpMetadata;

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Info>
          Devs: Click Download Metadata File button to see file error label
        </Info>
        <ConfigureServiceProvider samlIdPMetadata={samlIdPMetadata} />
      </ContextProvider>
    </MemoryRouter>
  );
};
