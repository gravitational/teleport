import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { AwsIcConfigureScim } from './Scim';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/SCIM',
};

export const ConfigureScim = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AwsIcConfigureScim />
      </ContextProvider>
    </MemoryRouter>
  );
};
