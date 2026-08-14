import { StoryObj } from '@storybook/react-vite';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import { GraphApiDetails } from './GraphApi';

export default {
  title: 'TeleportE/Integrations/Status/Entra/GraphApi',
};

export const Default: StoryObj = {
  render: () => {
    return render(
      '71cbeb2a-1b5b-44bd-909f-511a904a25b0',
      '9a5b7068-bb3c-4f76-9183-c0fbb6483b15',
      'ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS'
    );
  },
};

const render = (
  tenantId: string,
  entraAppId: string,
  credentialSource: string
) => {
  return (
    <ContextProvider ctx={createTeleportContextE()}>
      <GraphApiDetails
        tenantId={tenantId}
        entraAppId={entraAppId}
        credentialSource={credentialSource}
      />
    </ContextProvider>
  );
};
