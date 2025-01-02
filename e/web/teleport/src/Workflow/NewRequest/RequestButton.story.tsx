import { MemoryRouter } from 'react-router';

import { Flex } from 'design';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { useNewRequest } from 'e-teleport/Workflow/NewRequest/useNewRequest';
import { ContextProvider } from 'teleport';
import { App, AppSubKind } from 'teleport/services/apps';

import { IdentityCenterRequestButton as ICButton } from './RequestButton';

export default {
  title: 'TeleportE/AccessRequests/RequestButton',
};

export function IdentityCenterAccountRequestButton() {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter initialEntries={[{ pathname: '' }]}>
      <ContextProvider ctx={ctx}>
        <Flex
          mt={6}
          flexDirection="column"
          alignItems="center"
          justifyContent="center"
        >
          <Flex>
            <Button ctx={ctx} />
          </Flex>
        </Flex>
      </ContextProvider>
    </MemoryRouter>
  );
}

function Button({ ctx }: { ctx: TeleportEContext }) {
  const { addOrRemoveResources, addedResources } = useNewRequest(ctx);
  return (
    <ICButton
      agent={account1}
      addedResources={addedResources}
      addOrRemoveResources={addOrRemoveResources}
    />
  );
}

const account1: App = {
  kind: 'app',
  id: 'goteleport-local',
  subKind: AppSubKind.AwsIcAccount,
  name: 'goteleport-local',
  description: '',
  uri: 'https://console.aws.amazon.com',
  publicAddr: 'https://console.aws.amazon.com',
  fqdn: 'https://console.aws.amazon.com',
  clusterId: 'tele.dev',
  labels: [
    {
      name: 'teleport.dev/origin',
      value: 'aws-identity-center',
    },
  ],
  awsConsole: false,
  awsRoles: [],
  samlApp: false,
  userGroups: [],
  launchUrl: 'https://console.aws.amazon.com',
  friendlyName: 'goteleport-local',
  requiresRequest: true,
  permissionSets: [
    {
      name: 'AdministratorAccess',
      arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-25beafadd65e32d5',
      assignmentId: 'goteleport-local--administratoraccess',
    },
    {
      name: 'DataScientist',
      arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-93cddc3f6dae4744',
      assignmentId: 'goteleport-local--datascientist',
    },
    {
      name: 'NetworkAdministrator',
      arn: 'arn:aws:sso:::permissionSet/ssoins-8824eede0d4dd26a/ps-e644c1c111dc9656',
      assignmentId: 'goteleport-local--networkadministrator',
    },
  ],
};
