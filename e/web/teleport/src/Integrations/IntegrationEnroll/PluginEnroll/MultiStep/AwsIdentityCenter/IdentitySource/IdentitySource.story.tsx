import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { AwsIcConfigureIdentitySource } from './IdentitySource';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/IdentitySource',
};

export const ConfigureIdentitySource = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getMetadataXml = () =>
    Promise.resolve<string>('test file content');

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AwsIcConfigureIdentitySource />
      </ContextProvider>
    </MemoryRouter>
  );
};
