import { act, waitFor } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { MemoryRouter } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';

import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';
import {
  AccessMonitoringRule,
  AccessMonitoringRuleVersion,
} from 'e-teleport/services/accessmonitoringrule/types';
import { pluginsService } from 'e-teleport/services/plugins';
import { ContextProvider } from 'teleport';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';
import { Plugin } from 'teleport/services/integrations';
import { yamlService } from 'teleport/services/yaml';

import { AccessMonitoringRulesDialog } from './AccessMonitoringRulesDialog';

const mio = mockIntersectionObserver();

describe('AccessMonitoringRulesDialog', () => {
  beforeEach(() => {
    jest.spyOn(pluginsService, 'fetchPlugins').mockResolvedValue(plugins);

    jest.spyOn(yamlService, 'parse').mockResolvedValue(validRuleObject);
    jest.spyOn(yamlService, 'stringify').mockResolvedValue(ruleYaml);

    jest
      .spyOn(accessMonitoringRuleService, 'deleteAccessMonitoringRule')
      .mockResolvedValue(null);
    jest
      .spyOn(accessMonitoringRuleService, 'createAccessMonitoringRule')
      .mockResolvedValue({ object: createdRuleObject, yaml: '' });
    jest
      .spyOn(accessMonitoringRuleService, 'updateAccessMonitoringRule')
      .mockResolvedValue({
        object: { ...validRuleObject, metadata: { name: 'updated-rule' } },
        yaml: '',
      });
    jest
      .spyOn(
        accessMonitoringRuleService,
        'fetchAccessMonitoringRulesForAccessRequests'
      )
      .mockResolvedValue({
        startKey: '',
        rules: [
          { object: validRuleObject, yaml: ruleYaml },
          { object: invalidRuleObject, yaml: ruleYaml },
        ],
      } as any);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  // Async funcs are called separately on render.
  // One from useKeyBasedPagination hook.
  // One for fetching plugins.
  // We need to wait for both to finish.
  async function waitForAllAsyncCalls() {
    // Wait for the plugins table to show up.
    expect(await screen.findByRole('table')).toBeVisible();
    expect(pluginsService.fetchPlugins).toHaveBeenCalledTimes(1);

    act(mio.enterAll); // trigger the isIntersecting of IntersectionObserver
    expect(
      accessMonitoringRuleService.fetchAccessMonitoringRulesForAccessRequests
    ).toHaveBeenCalledTimes(1);
  }

  test('fetched rules and plugins are listed', async () => {
    render(<Component />);
    await waitForAllAsyncCalls();

    await screen.findAllByText(/plugin-name/i);
    expect(screen.getAllByText(/plugin-name/i)).toHaveLength(2);
    expect(screen.getAllByText(/view/i)).toHaveLength(2); // plugins don't have view buttons
    expect(screen.getAllByText(/fallback slack/i)).toHaveLength(2);
  });

  test('no plugins should render info to add a plugin first', async () => {
    jest.spyOn(pluginsService, 'fetchPlugins').mockResolvedValue([]);

    render(<Component />);
    await waitForAllAsyncCalls();

    await screen.findAllByText(/plugin-name/i);

    await userEvent.click(
      screen.getByRole('button', { name: /create new access automation rule/i })
    );

    await userEvent.click(
      screen.getByRole('menuitem', { name: /notification routing rule/i })
    );

    expect(
      screen.getByText(/create new notification routing rule/i)
    ).toBeInTheDocument();

    expect(screen.getByText(/enroll an integration/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create rule/i })).toBeDisabled();
  });

  test('deleting a rule, removes it from the cached list', async () => {
    render(<Component />);
    await waitForAllAsyncCalls();

    await screen.findAllByText(/plugin-name/i);

    const btns = screen.getAllByRole('button', { name: /view/i });
    await userEvent.click(btns[0]);
    expect(screen.getByTestId('standard')).toBeInTheDocument();

    await userEvent.click(screen.getByTestId('delete'));

    await screen.findByText(/delete rule\?/i);
    await userEvent.click(
      screen.getByRole('button', { name: /yes, delete rule/i })
    );

    expect(screen.getAllByRole('button', { name: /view/i })).toHaveLength(1);
    expect(screen.queryByText(/name-valid-/i)).not.toBeInTheDocument();
    expect(screen.getByText(/name-invalid-/i)).toBeInTheDocument();

    // sidebar is closed after delete.
    expect(screen.queryByTestId('standard')).not.toBeInTheDocument();
  });

  test('creating a rule, adds it to the cached list', async () => {
    render(<Component />);
    await waitForAllAsyncCalls();

    await screen.findAllByText(/plugin-name/i);
    expect(screen.getAllByRole('button', { name: /view/i })).toHaveLength(2);

    await userEvent.click(
      screen.getByRole('button', { name: /create new access automation rule/i })
    );

    await userEvent.click(
      screen.getByRole('menuitem', { name: /notification routing rule/i })
    );

    await userEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    await userEvent.click(screen.getByRole('button', { name: /create rule/i }));

    expect(screen.getAllByRole('button', { name: /view/i })).toHaveLength(3);
    expect(screen.getByText(/created-rule/i)).toBeInTheDocument();

    // sidebar is closed after create.
    expect(screen.queryByTestId('yaml')).not.toBeInTheDocument();
  });

  test('create automatic review rule', async () => {
    render(<Component />);
    await waitForAllAsyncCalls();

    await userEvent.click(
      screen.getByRole('button', { name: /create new access automation rule/i })
    );

    await userEvent.click(
      screen.getByRole('menuitem', { name: /automatic review rule/i })
    );

    await userEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    await userEvent.click(screen.getByRole('button', { name: /create rule/i }));

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /view/i })).toHaveLength(3);
    });
    expect(screen.getByText(/created-rule/i)).toBeInTheDocument();

    // sidebar is closed after create.
    expect(screen.queryByTestId('yaml')).not.toBeInTheDocument();
  });

  test('viewing a valid rule, defaults to standard editor', async () => {
    jest
      .spyOn(
        accessMonitoringRuleService,
        'fetchAccessMonitoringRulesForAccessRequests'
      )
      .mockResolvedValue({
        startKey: '',
        rules: [{ object: validRuleObject, yaml: ruleYaml }],
      } as any);

    render(<Component />);
    await waitForAllAsyncCalls();
    await screen.findAllByText(/plugin-name/i);

    expect(
      screen.queryByRole('tab', { name: /standard/i })
    ).not.toBeInTheDocument();

    // default to standard editor when viewing
    await userEvent.click(screen.getByRole('button', { name: /view/i }));
    expect(screen.getByRole('tab', { name: /standard/i })).toHaveClass(
      'selected'
    );
    expect(screen.getByTestId('standard')).toBeInTheDocument();
    expect(
      screen.queryByText(/enroll an integration/i)
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/reset to standard/i)).not.toBeInTheDocument();

    // click to view yaml
    await userEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    expect(screen.getByRole('tab', { name: /yaml/i })).toHaveClass('selected');
    expect(screen.getByTestId('yaml')).toBeInTheDocument();
  });

  test('viewing a invalid rule defaults to yaml editor and renders reset button on switching to standard', async () => {
    jest.spyOn(yamlService, 'parse').mockResolvedValue(invalidRuleObject);
    jest
      .spyOn(
        accessMonitoringRuleService,
        'fetchAccessMonitoringRulesForAccessRequests'
      )
      .mockResolvedValue({
        startKey: '',
        rules: [{ object: invalidRuleObject, yaml: ruleYaml }],
      } as any);

    render(<Component />);
    await waitForAllAsyncCalls();
    await screen.findAllByText(/plugin-name/i);

    expect(
      screen.queryByRole('tab', { name: /standard/i })
    ).not.toBeInTheDocument();

    // defaults to yaml editor when viewing
    await userEvent.click(screen.getByRole('button', { name: /view/i }));
    expect(screen.getByRole('tab', { name: /yaml/i })).toHaveClass('selected');
    expect(screen.getByTestId('yaml')).toBeInTheDocument();

    // clicking to view standard should render reset button
    await userEvent.click(screen.getByRole('tab', { name: /standard/i }));
    expect(screen.getByRole('tab', { name: /standard/i })).toHaveClass(
      'selected'
    );
    expect(screen.getByTestId('standard')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /update rule/i })).toBeDisabled();

    await userEvent.click(
      screen.getByRole('button', { name: /reset to standard/i })
    );

    expect(
      screen.queryByRole('button', { name: /reset to standard/i })
    ).not.toBeInTheDocument();
  });
});

const ruleYaml = `kind: access_monitoring_rule
metadata:
name: sdfssd
spec:
condition: some-condition
notification:
name: mattermost
recipients:
- apple
- banana
- carrot
subjects:
- access_request
version: v1`;

const validRuleObject: AccessMonitoringRule = {
  kind: 'access_monitoring_rule',
  version: AccessMonitoringRuleVersion.V1,
  metadata: { name: 'name-valid-default-to-standard-editor' },
  spec: {
    subjects: ['access_request'],
    condition: '',
    notification: {
      name: 'plugin-name',
      recipients: ['apple', 'banana', 'carrot'],
    },
  },
};

const invalidRuleObject: AccessMonitoringRule = {
  kind: 'access_monitoring_rule',
  version: AccessMonitoringRuleVersion.V1,
  metadata: { name: 'name-invalid-fields-default-to-yaml-editor' },
  spec: {
    subjects: ['access_request'],
    condition: 'invalid field',
    notification: {
      name: 'plugin-name',
      recipients: ['apple', 'banana', 'carrot'],
    },
  },
};

const createdRuleObject: AccessMonitoringRule = {
  kind: 'access_monitoring_rule',
  version: AccessMonitoringRuleVersion.V1,
  metadata: { name: 'created-rule' },
  spec: {
    subjects: ['access_request'],
    condition: '',
    notification: {
      name: 'plugin-name',
      recipients: [],
    },
  },
};

const plugins: Plugin[] = [
  {
    resourceType: 'plugin',
    name: 'slack-plugin1',
    details: '',
    statusCode: 0,
    kind: 'slack',
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'slack-plugin2',
    details: '',
    statusCode: 0,
    kind: 'slack',
    spec: {},
  },
];

const Component = ({ noAccess = false }: { noAccess?: boolean }) => {
  const ctx = createTeleportContext();
  ctx.storeUser.state.acl = getAcl({ noAccess: noAccess });
  ctx.resourceService.fetchRoles = jest
    .fn()
    .mockResolvedValue({ items: [{ name: 'role1' }] });
  return (
    <MemoryRouter initialEntries={[{ pathname: '' }]}>
      <ContextProvider ctx={ctx}>
        <AccessMonitoringRulesDialog
          onClose={() => null}
          transitionState="entered"
        />
      </ContextProvider>
    </MemoryRouter>
  );
};
