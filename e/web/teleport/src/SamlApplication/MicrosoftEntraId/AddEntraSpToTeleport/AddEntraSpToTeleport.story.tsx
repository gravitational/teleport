import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { resourceSpecSamlGeneric } from 'e-teleport/SamlApplication/fixtures';
import { SamlApplicationProvider } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';

import { AddEntraSpToTeleport as AddEntraSpToTeleportComponent } from './AddEntraSpToTeleport';

export default {
  title: 'TeleportE/SamlApplication/MicrosoftEntraId',
};

export const AddEntraSpToTeleport = () => {
  const ctx = createTeleportContextE();
  return (
    <RequiredDiscoverProviders
      agentMeta={{}}
      resourceSpec={resourceSpecSamlGeneric}
      teleportCtx={ctx}
    >
      <SamlApplicationProvider>
        <AddEntraSpToTeleportComponent />
      </SamlApplicationProvider>
    </RequiredDiscoverProviders>
  );
};
