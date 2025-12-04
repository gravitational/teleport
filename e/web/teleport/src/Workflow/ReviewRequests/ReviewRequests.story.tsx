import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { getAcl } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import Workflow from './ReviewRequests';

export default {
  title: 'TeleportE/AccessRequests',
};

export const WithReadAccess = () => {
  return <Component noAccess={false} />;
};

export const WithNoReadAccess = () => {
  return <Component noAccess={true} />;
};

const Component = ({ noAccess = false }: { noAccess?: boolean }) => {
  const ctx = createTeleportContextE();
  ctx.storeUser.state.acl = getAcl({ noAccess: noAccess });
  ctx.workflowService.fetchAccessRequests = async () => {
    return { agents: [], requests: [], startKey: '' };
  };
  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Workflow />
    </TeleportProviderBasic>
  );
};
