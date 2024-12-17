import { MemoryRouter } from 'react-router';

import { DownloadMetadata as DownloadMetadataComponent } from './DownloadMetadata';

export default {
  title: 'TeleportE/Discover/SAML Application',
};

export const DownloadMetadataGrafana = () => {
  return (
    <MemoryRouter>
      <DownloadMetadataComponent nextStep={() => null} prevStep={() => null} />
    </MemoryRouter>
  );
};
