/* eslint-disable testing-library/no-node-access */
import { within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';
import { getEmptyResourceState } from 'shared/components/AccessRequests/NewRequest';
import { getResourceIDString } from 'shared/services/accessRequests';
import { AwsRole } from 'shared/services/apps';
import { ComponentFeatureID } from 'shared/utils/componentFeatures';

import { ResourcePrincipalSet } from 'teleport/services/agents';
import { App } from 'teleport/services/apps';
import { Node } from 'teleport/services/nodes';

import { AppAwsRoleMenu, NodeSshLoginMenu } from './RequestButton';

type AwsConsoleApp = App & { awsConsole: true; awsRoles?: AwsRole[] };

const loginPrincipals = (
  granted: string[],
  requestable: string[] = []
): ResourcePrincipalSet[] => [
  { principalType: 'logins', granted, requestable },
];

const makeAwsApp = (overrides: Partial<App> = {}): AwsConsoleApp =>
  ({
    id: 'test-app',
    name: 'test-app',
    description: '',
    uri: 'https://console.aws.amazon.com',
    publicAddr: 'test-app.example.com',
    labels: [],
    clusterId: 'cluster-1',
    launchUrl: '/launch',
    fqdn: 'test-app.example.com',
    awsRoles: [],
    userGroups: [],
    samlApp: false,
    ...overrides,
    kind: 'app',
    awsConsole: true,
    supportedFeatureIds: [ComponentFeatureID.ResourceConstraintsV1],
  }) satisfies AwsConsoleApp;

const makeSshNode = (overrides: Partial<Node> = {}): Node =>
  ({
    id: 'node-1',
    clusterId: 'cluster-1',
    hostname: 'test-node',
    labels: [],
    addr: '10.0.0.1',
    tunnel: false,
    subKind: 'teleport',
    sshLogins: [],
    principals: [],
    ...overrides,
    kind: 'node',
    supportedFeatureIds: [ComponentFeatureID.ResourceConstraintsSshV1],
  }) satisfies Node;

const emptyResources = getEmptyResourceState();

describe('AppAwsRoleMenu', () => {
  const baseProps = {
    addedResources: emptyResources,
    addedResourceConstraints: {},
    addOrRemoveResources: jest.fn(),
    setResourceConstraints: jest.fn(),
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders disabled button when no IAM roles exist', () => {
    const agent = makeAwsApp({ awsRoles: [] });
    render(<AppAwsRoleMenu {...baseProps} agent={agent} />);

    expect(screen.getByRole('button', { name: 'Launch' })).toBeDisabled();
  });

  it('renders a dropdown when a single granted role exists', async () => {
    const user = userEvent.setup();
    const agent = makeAwsApp({
      awsRoles: [
        {
          name: 'admin',
          arn: 'arn:aws:iam::123:role/admin',
          display: 'admin',
          accountId: '123',
        },
      ],
    });
    render(<AppAwsRoleMenu {...baseProps} agent={agent} />);

    await user.click(screen.getByRole('button', { name: /launch/i }));
    expect(screen.getByText(/admin/)).toBeVisible();
  });

  it('renders a menu with granted and requestable sections', async () => {
    const user = userEvent.setup();
    const agent = makeAwsApp({
      awsRoles: [
        {
          name: 'granted-role',
          arn: 'arn:aws:iam::111:role/granted',
          display: 'granted-role',
          accountId: '111',
        },
        {
          name: 'requestable-role',
          arn: 'arn:aws:iam::222:role/requestable',
          display: 'requestable-role',
          accountId: '222',
          requiresRequest: true,
        },
        {
          name: 'another-requestable',
          arn: 'arn:aws:iam::333:role/another',
          display: 'another-requestable',
          accountId: '333',
          requiresRequest: true,
        },
      ],
    });

    render(<AppAwsRoleMenu {...baseProps} agent={agent} />);

    await user.click(screen.getByRole('button', { name: /launch/i }));

    expect(screen.getByText('Connect:')).toBeVisible();
    expect(screen.getByText('Request Access:')).toBeVisible();
    expect(screen.getByText(/granted-role/)).toBeVisible();
    expect(screen.getByText(/requestable-role/)).toBeVisible();
    expect(screen.getByText(/another-requestable/)).toBeVisible();
  });

  it('toggles a requestable item when clicked', async () => {
    const user = userEvent.setup();
    const setResourceConstraints = jest.fn();
    const addOrRemoveResources = jest.fn();
    const agent = makeAwsApp({
      awsRoles: [
        {
          name: 'role-a',
          arn: 'arn:aws:iam::111:role/a',
          display: 'role-a',
          accountId: '111',
          requiresRequest: true,
        },
        {
          name: 'role-b',
          arn: 'arn:aws:iam::222:role/b',
          display: 'role-b',
          accountId: '222',
          requiresRequest: true,
        },
      ],
    });

    render(
      <AppAwsRoleMenu
        {...baseProps}
        agent={agent}
        addOrRemoveResources={addOrRemoveResources}
        setResourceConstraints={setResourceConstraints}
      />
    );

    await user.click(screen.getByRole('button', { name: /request/i }));
    await user.click(screen.getByText(/role-a/));

    expect(setResourceConstraints).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        aws_console: { role_arns: ['arn:aws:iam::111:role/a'] },
      })
    );
  });

  it('shows search bar when more than 2 choices exist', async () => {
    const user = userEvent.setup();
    const agent = makeAwsApp({
      awsRoles: [
        {
          name: 'alpha',
          arn: 'arn:1',
          display: 'alpha',
          accountId: '1',
          requiresRequest: true,
        },
        {
          name: 'beta',
          arn: 'arn:2',
          display: 'beta',
          accountId: '2',
          requiresRequest: true,
        },
        {
          name: 'gamma',
          arn: 'arn:3',
          display: 'gamma',
          accountId: '3',
          requiresRequest: true,
        },
      ],
    });

    render(<AppAwsRoleMenu {...baseProps} agent={agent} />);
    await user.click(screen.getByRole('button'));

    const searchInput = screen.getByPlaceholderText('Search IAM roles...');
    expect(searchInput).toBeInTheDocument();
  });

  it('filters items by search and hides non-matching items', async () => {
    const user = userEvent.setup();
    const agent = makeAwsApp({
      awsRoles: [
        {
          name: 'alpha',
          arn: 'arn:1',
          display: 'alpha',
          accountId: '1',
          requiresRequest: true,
        },
        {
          name: 'beta',
          arn: 'arn:2',
          display: 'beta',
          accountId: '2',
          requiresRequest: true,
        },
        {
          name: 'gamma',
          arn: 'arn:3',
          display: 'gamma',
          accountId: '3',
          requiresRequest: true,
        },
      ],
    });

    render(<AppAwsRoleMenu {...baseProps} agent={agent} />);
    await user.click(screen.getByRole('button'));
    await user.type(
      screen.getByPlaceholderText('Search IAM roles...'),
      'alpha'
    );

    // 'alpha' should be visible, 'beta' and 'gamma' should be hidden
    expect(screen.getByText(/alpha/)).toBeVisible();
    expect(screen.getByText(/beta/).closest('label')).toHaveAttribute(
      'aria-hidden',
      'true'
    );
    expect(screen.getByText(/gamma/).closest('label')).toHaveAttribute(
      'aria-hidden',
      'true'
    );
  });

  it('select all toggles all visible requestable items', async () => {
    const user = userEvent.setup();
    const setResourceConstraints = jest.fn();
    const addOrRemoveResources = jest.fn();
    const agent = makeAwsApp({
      awsRoles: [
        {
          name: 'alpha',
          arn: 'arn:1',
          display: 'alpha',
          accountId: '1',
          requiresRequest: true,
        },
        {
          name: 'beta',
          arn: 'arn:2',
          display: 'beta',
          accountId: '2',
          requiresRequest: true,
        },
        {
          name: 'gamma',
          arn: 'arn:3',
          display: 'gamma',
          accountId: '3',
          requiresRequest: true,
        },
      ],
    });

    render(
      <AppAwsRoleMenu
        {...baseProps}
        agent={agent}
        addOrRemoveResources={addOrRemoveResources}
        setResourceConstraints={setResourceConstraints}
      />
    );

    await user.click(screen.getByRole('button'));
    await user.click(screen.getByText('Select All'));

    expect(setResourceConstraints).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        aws_console: {
          role_arns: expect.arrayContaining(['arn:1', 'arn:2', 'arn:3']),
        },
      })
    );
  });

  it('sorts roles by account ID then by name', async () => {
    const user = userEvent.setup();
    const agent = makeAwsApp({
      awsRoles: [
        {
          name: 'zeta',
          arn: 'arn:z',
          display: 'zeta',
          accountId: '222',
          requiresRequest: true,
        },
        {
          name: 'alpha',
          arn: 'arn:a',
          display: 'alpha',
          accountId: '111',
          requiresRequest: true,
        },
        {
          name: 'beta',
          arn: 'arn:b',
          display: 'beta',
          accountId: '111',
          requiresRequest: true,
        },
      ],
    });

    render(<AppAwsRoleMenu {...baseProps} agent={agent} />);
    await user.click(screen.getByRole('button'));

    const section = screen.getByText('Request Access:').parentElement;
    const labels = within(section).getAllByRole('checkbox');
    // First item is "Select All", so skip it, then expect sorted: 111:alpha, 111:beta, 222:zeta
    const labelTexts = labels
      .slice(1)
      .map(cb => cb.closest('label')?.textContent);

    expect(labelTexts[0]).toContain('alpha');
    expect(labelTexts[1]).toContain('beta');
    expect(labelTexts[2]).toContain('zeta');
  });
});

describe('NodeSshLoginMenu', () => {
  const baseProps = {
    addedResources: emptyResources,
    addedResourceConstraints: {},
    addOrRemoveResources: jest.fn(),
    setResourceConstraints: jest.fn(),
    clusterId: 'cluster-1',
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders disabled button when no logins exist', () => {
    const agent = makeSshNode({ principals: [] });
    render(<NodeSshLoginMenu {...baseProps} agent={agent} />);

    expect(screen.getByRole('button', { name: 'Connect' })).toBeDisabled();
  });

  it('renders a dropdown when a single granted login exists', async () => {
    const user = userEvent.setup();
    const agent = makeSshNode({
      principals: loginPrincipals(['ubuntu']),
    });
    render(<NodeSshLoginMenu {...baseProps} agent={agent} />);

    await user.click(screen.getByRole('button', { name: /connect/i }));
    expect(screen.getByText('ubuntu')).toBeVisible();
  });

  it('renders menu with Connect and Request sections', async () => {
    const user = userEvent.setup();
    const agent = makeSshNode({
      principals: loginPrincipals(['ubuntu'], ['ec2-user', 'admin']),
    });

    render(<NodeSshLoginMenu {...baseProps} agent={agent} />);
    await user.click(screen.getByRole('button'));

    expect(screen.getByText('Connect:')).toBeVisible();
    expect(screen.getByText('Request Access:')).toBeVisible();
    expect(screen.getByText('ubuntu')).toBeVisible();
    expect(screen.getByText('ec2-user')).toBeVisible();
    expect(screen.getByText('admin')).toBeVisible();
  });

  it('sorts logins alphabetically with root first', async () => {
    const user = userEvent.setup();
    const agent = makeSshNode({
      principals: loginPrincipals([], ['zulu', 'root', 'alpha']),
    });

    render(<NodeSshLoginMenu {...baseProps} agent={agent} />);
    await user.click(screen.getByRole('button'));

    const section = screen.getByText('Request Access:').parentElement;
    const labels = within(section).getAllByRole('checkbox');
    const loginTexts = labels
      .slice(1) // skip Select All checkbox
      .map(cb => cb.closest('label')?.textContent);

    expect(loginTexts[0]).toBe('root');
    expect(loginTexts[1]).toBe('alpha');
    expect(loginTexts[2]).toBe('zulu');
  });

  it('toggles login selection via checkbox', async () => {
    const user = userEvent.setup();
    const setResourceConstraints = jest.fn();
    const addOrRemoveResources = jest.fn();
    const agent = makeSshNode({
      principals: loginPrincipals([], ['ubuntu', 'ec2-user']),
    });

    render(
      <NodeSshLoginMenu
        {...baseProps}
        agent={agent}
        addOrRemoveResources={addOrRemoveResources}
        setResourceConstraints={setResourceConstraints}
      />
    );

    await user.click(screen.getByRole('button'));
    await user.click(screen.getByText('ubuntu'));

    expect(setResourceConstraints).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        ssh: { logins: ['ubuntu'] },
      })
    );
  });

  it('shows no logins message when search matches nothing', async () => {
    const user = userEvent.setup();
    const agent = makeSshNode({
      principals: loginPrincipals([], ['ubuntu', 'ec2-user', 'admin']),
    });

    render(<NodeSshLoginMenu {...baseProps} agent={agent} />);
    await user.click(screen.getByRole('button'));
    await user.type(
      screen.getByPlaceholderText('Search logins...'),
      'nonexistent'
    );

    expect(screen.getByText('No logins found')).toBeVisible();
  });

  it('hides Select All when only 1 visible requestable item remains', async () => {
    const user = userEvent.setup();
    const agent = makeSshNode({
      principals: loginPrincipals([], ['ubuntu', 'ec2-user', 'admin']),
    });

    render(<NodeSshLoginMenu {...baseProps} agent={agent} />);
    await user.click(screen.getByRole('button'));
    await user.type(screen.getByPlaceholderText('Search logins...'), 'ubuntu');

    expect(screen.getByText('Select All').closest('label')).toHaveAttribute(
      'aria-hidden',
      'true'
    );
  });

  it('deselects all when select all is clicked and all are selected', async () => {
    const user = userEvent.setup();
    const setResourceConstraints = jest.fn();
    const addOrRemoveResources = jest.fn();
    const agent = makeSshNode({
      principals: loginPrincipals([], ['ubuntu', 'ec2-user']),
    });

    const key = getResourceIDString({
      cluster: 'cluster-1',
      kind: 'node',
      name: 'node-1',
    });

    render(
      <NodeSshLoginMenu
        {...baseProps}
        agent={agent}
        addedResources={{ ...emptyResources, node: { 'node-1': 'node-1' } }}
        addedResourceConstraints={{
          [key]: { ssh: { logins: ['ubuntu', 'ec2-user'] } },
        }}
        addOrRemoveResources={addOrRemoveResources}
        setResourceConstraints={setResourceConstraints}
      />
    );

    await user.click(screen.getByRole('button'));
    await user.click(screen.getByText('Select All'));

    // Should deselect all
    expect(setResourceConstraints).toHaveBeenCalledWith(
      expect.any(String),
      undefined
    );
  });
});
