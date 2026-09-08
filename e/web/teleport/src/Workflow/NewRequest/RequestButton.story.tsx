import type { Meta, StoryObj } from '@storybook/react-vite';
import { PropsWithChildren, ReactElement } from 'react';
import { MemoryRouter } from 'react-router';

import { Flex } from 'design';
import { AppSubKind } from 'shared/services';
import { AwsRole } from 'shared/services/apps';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { useNewRequest } from 'e-teleport/Workflow/NewRequest/useNewRequest';
import { ContextProvider } from 'teleport';
import { App } from 'teleport/services/apps';
import { Node } from 'teleport/services/nodes';

import {
  AppAwsRoleMenu,
  IdentityCenterRequestButton as ICButton,
  NodeSshLoginMenu,
} from './RequestButton';

export default {
  title: 'TeleportE/AccessRequests/RequestButton',
} satisfies Meta;

type Story = StoryObj;

const StoryWrapper = ({
  children,
  ctx,
}: PropsWithChildren<{ ctx: TeleportEContext }>) => (
  <MemoryRouter initialEntries={[{ pathname: '' }]}>
    <ContextProvider ctx={ctx}>
      <Flex
        mt={6}
        flexDirection="column"
        justifyContent="center"
        alignItems="center"
      >
        <Flex>{children}</Flex>
      </Flex>
    </ContextProvider>
  </MemoryRouter>
);

const AWSConsoleInner = ({
  ctx,
  agent,
}: {
  ctx: TeleportEContext;
  agent: typeof baseConsoleApp;
}) => {
  const {
    addOrRemoveResources,
    addedResources,
    addedResourceConstraints,
    setResourceConstraints,
  } = useNewRequest(ctx);
  return (
    <AppAwsRoleMenu
      agent={agent}
      addedResources={addedResources}
      addOrRemoveResources={addOrRemoveResources}
      addedResourceConstraints={addedResourceConstraints}
      setResourceConstraints={setResourceConstraints}
      requestStarted={!!Object.keys(addedResourceConstraints).length}
    />
  );
};

const SSHNodeInner = ({
  ctx,
  agent,
}: {
  ctx: TeleportEContext;
  agent: Node;
}) => {
  const {
    addOrRemoveResources,
    addedResources,
    addedResourceConstraints,
    setResourceConstraints,
  } = useNewRequest(ctx);
  return (
    <NodeSshLoginMenu
      agent={agent}
      addedResources={addedResources}
      addOrRemoveResources={addOrRemoveResources}
      addedResourceConstraints={addedResourceConstraints}
      setResourceConstraints={setResourceConstraints}
      clusterId={agent.clusterId}
      requestStarted={!!Object.keys(addedResourceConstraints).length}
    />
  );
};

const ICInner = ({ ctx }: { ctx: TeleportEContext }) => {
  const { addOrRemoveResources, addedResources } = useNewRequest(ctx);
  return (
    <ICButton
      agent={account1}
      addedResources={addedResources}
      addOrRemoveResources={addOrRemoveResources}
    />
  );
};

function renderWithCtx(inner: (ctx: TeleportEContext) => ReactElement) {
  const ctx = createTeleportContextE();
  return <StoryWrapper ctx={ctx}>{inner(ctx)}</StoryWrapper>;
}

const AWSConsoleOneGranted: Story = {
  name: 'AWS Console / One Granted Role',
  render: () =>
    renderWithCtx(ctx => (
      <AWSConsoleInner ctx={ctx} agent={consoleAppWith([readOnlyRole])} />
    )),
};

const AWSConsoleOneGrantedOneRequestable: Story = {
  name: 'AWS Console / One Granted + One Requestable',
  render: () =>
    renderWithCtx(ctx => (
      <AWSConsoleInner
        ctx={ctx}
        agent={consoleAppWith([readOnlyRole, adminRole])}
      />
    )),
};

const AWSConsoleOneRequestable: Story = {
  name: 'AWS Console / One Requestable Role',
  render: () =>
    renderWithCtx(ctx => (
      <AWSConsoleInner ctx={ctx} agent={consoleAppWith([adminRole])} />
    )),
};

const SSHNodeOneGranted: Story = {
  name: 'SSH Node / One Granted Login',
  render: () =>
    renderWithCtx(ctx => (
      <SSHNodeInner
        ctx={ctx}
        agent={sshNodeWith([{ login: 'ubuntu', requiresRequest: false }])}
      />
    )),
};

const SSHNodeOneGrantedOneRequestable: Story = {
  name: 'SSH Node / One Granted + One Requestable',
  render: () =>
    renderWithCtx(ctx => (
      <SSHNodeInner
        ctx={ctx}
        agent={sshNodeWith([
          { login: 'ubuntu', requiresRequest: false },
          { login: 'root', requiresRequest: true },
        ])}
      />
    )),
};

const SSHNodeOneRequestable: Story = {
  name: 'SSH Node / One Requestable Login',
  render: () =>
    renderWithCtx(ctx => (
      <SSHNodeInner
        ctx={ctx}
        agent={sshNodeWith([{ login: 'root', requiresRequest: true }])}
      />
    )),
};

const SSHNodeManyLogins: Story = {
  name: 'SSH Node / Many Logins',
  render: () =>
    renderWithCtx(ctx => (
      <SSHNodeInner
        ctx={ctx}
        agent={sshNodeWith([
          { login: 'web' },
          { login: 'alice' },
          { login: 'deploy', requiresRequest: true },
          { login: 'ci-runner', requiresRequest: true },
          { login: 'ec2-user', requiresRequest: true },
          { login: 'backup-agent', requiresRequest: true },
          { login: 'root', requiresRequest: true },
          { login: 'ubuntu', requiresRequest: true },
          { login: 'vault-sync', requiresRequest: true },
          { login: 'devops', requiresRequest: true },
        ])}
      />
    )),
};

const IdentityCenterAccount: Story = {
  render: () => renderWithCtx(ctx => <ICInner ctx={ctx} />),
};

export {
  AWSConsoleOneGranted,
  AWSConsoleOneGrantedOneRequestable,
  AWSConsoleOneRequestable,
  SSHNodeOneGranted,
  SSHNodeOneGrantedOneRequestable,
  SSHNodeOneRequestable,
  SSHNodeManyLogins,
  IdentityCenterAccount,
};

// Fixtures

const readOnlyRole: AwsRole = {
  name: 'ReadOnly',
  display: 'ReadOnly',
  accountId: '123456789012',
  arn: 'arn:aws:iam::123456789012:role/ReadOnlyAccess',
};

const adminRole: AwsRole = {
  name: 'Admin',
  display: 'Admin',
  accountId: '123456789012',
  arn: 'arn:aws:iam::123456789012:role/Admin',
  requiresRequest: true,
};

const baseConsoleApp = {
  kind: 'app' as const,
  id: 'aws-console',
  name: 'awsconsole',
  description: '',
  uri: 'https://console.aws.amazon.com',
  publicAddr: 'https://aws-console.tele.dev',
  fqdn: 'https://aws-console.tele.dev',
  clusterId: 'tele.dev',
  labels: [{ name: 'teleport.dev/origin', value: 'aws-identity-center' }],
  awsConsole: true as const,
  samlApp: false,
  launchUrl: 'https://aws-console.tele.dev',
  userGroups: [],
  awsRoles: [] as AwsRole[],
};

function consoleAppWith(roles: AwsRole[]) {
  return { ...baseConsoleApp, awsRoles: roles };
}

const baseSSHNode: Node = {
  kind: 'node',
  id: 'some-server',
  clusterId: 'tele.dev',
  hostname: 'someserver',
  labels: [{ name: 'teleport.dev/origin', value: 'config' }],
  addr: 'someserver.tele.dev',
  subKind: 'teleport',
  tunnel: false,
  sshLogins: [],
};

function sshNodeWith(logins: { login: string; requiresRequest?: boolean }[]) {
  return {
    ...baseSSHNode,
    principals: [
      {
        principalType: 'logins' as const,
        granted: logins.filter(l => !l.requiresRequest).map(l => l.login),
        requestable: logins.filter(l => l.requiresRequest).map(l => l.login),
      },
    ],
  };
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
  labels: [{ name: 'teleport.dev/origin', value: 'aws-identity-center' }],
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
