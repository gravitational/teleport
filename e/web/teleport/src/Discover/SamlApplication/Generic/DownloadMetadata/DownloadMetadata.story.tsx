import React from 'react';
import { MemoryRouter } from 'react-router';

import { idpMetadata } from 'e-teleport/Discover/SamlApplication/shared/fixtures';

import { ConfigureServiceProvider } from './DownloadMetadata';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';

export default {
  title: 'TeleportE/Discover/SAML Application',
};

export const DownloadMetadata = () => {
  const samlIdPMetadata: SAMLIdPMetadataResponse = idpMetadata;

  return (
    <MemoryRouter>
      <ConfigureServiceProvider samlIdPMetadata={samlIdPMetadata} />
    </MemoryRouter>
  );
};
