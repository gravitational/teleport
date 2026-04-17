/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { QueryClientProvider } from '@tanstack/react-query';
import { ComponentProps, PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
  within,
} from 'design/utils/testing';
import Validation from 'shared/components/Validation';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ResourcesResponse, UnifiedResource } from 'teleport/services/agents';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { fetchUnifiedResourcesSuccess } from 'teleport/test/helpers/resources';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { trackingTester } from '../Shared/trackingTester';
import { TrackingProvider } from '../Shared/useTracking';
import { KubernetesLabelsSelect } from './KubernetesLabelsSelect';

enableMswServer();

beforeEach(() => {
  server.use(userEventCaptureSuccess());
});

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.clearAllMocks();
});

afterAll(() => {
  jest.resetAllMocks();
});

describe('KubernetesLabelsSelect', () => {
  test('renders', async () => {
    renderComponent({
      props: {
        selected: [
          {
            name: 'foo',
            values: ['bar'],
          },
          {
            name: 'env',
            values: ['*'],
          },
        ],
      },
    });

    expect(screen.getByText('foo: bar')).toBeInTheDocument();
    expect(screen.getByText('env: *')).toBeInTheDocument();
  });

  test('empty state', async () => {
    renderComponent();

    expect(screen.getByText('No labels selected.')).toBeInTheDocument();
  });

  test('edit action', async () => {
    const tracking = trackingTester();

    withListUnifiedResourcesSuccess();

    const { user } = renderComponent();

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    expect(
      within(modal).getByText('Select one or more labels to configure access.')
    ).toBeInTheDocument();

    tracking.assertSection(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONFIGURE_ACCESS',
      'INTEGRATION_ENROLL_SECTION_MWIGHAK8S_KUBERNETES_LABEL_PICKER'
    );
  });

  test('cluster list empty', async () => {
    withListUnifiedResourcesSuccess({
      response: {
        agents: [],
      },
    });

    const { user } = renderComponent();

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    expect(within(modal).getByText('No clusters found')).toBeInTheDocument();
  });

  test('cluster list', async () => {
    withListUnifiedResourcesSuccess();

    const { user } = renderComponent();

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    expect(
      within(modal).getByText('kube-lon-dev-01.example.com')
    ).toBeInTheDocument();
    expect(
      within(modal).getByText('kube-lon-prod-01.example.com')
    ).toBeInTheDocument();
    expect(
      within(modal).getByText('kube-lon-staging-01.example.com')
    ).toBeInTheDocument();
    expect(
      within(modal).getByText('kube-temp-01.example.com')
    ).toBeInTheDocument();
  });

  test('zero selected labels', async () => {
    withListUnifiedResourcesSuccess();

    const { user } = renderComponent({
      props: {
        selected: [],
      },
    });

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    expect(within(modal).getByText('Selected Labels (0)')).toBeInTheDocument();
  });

  test('selected labels', async () => {
    withListUnifiedResourcesSuccess();

    const { user } = renderComponent({
      props: {
        selected: [
          {
            name: 'test-1',
            values: ['one', 'two', 'three'],
          },
          {
            name: 'test-2',
            values: ['*'],
          },
          {
            name: 'test-*',
            values: ['^(alpha|beta)$'],
          },
        ],
      },
    });

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    expect(
      within(modal).getByRole('heading', { name: 'Selected Labels (3)' })
    ).toBeInTheDocument();
  });

  test('add label', async () => {
    withListUnifiedResourcesSuccess();

    const { user } = renderComponent();

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    expect(
      within(modal).getByRole('heading', { name: 'Selected Labels (0)' })
    ).toBeInTheDocument();

    const item = screen.getByTestId('cluster-item-kube-lon-dev-01.example.com');
    const addLabel = within(item).getByTestId('label-action-env: dev-add');
    await user.click(addLabel);

    expect(
      within(modal).getByRole('heading', { name: 'Selected Labels (1)' })
    ).toBeInTheDocument();

    const selectedSection = within(modal)
      .getByRole('heading', { name: 'Selected Labels (1)' })
      .closest('div');
    expect(within(selectedSection!).getByText('env: dev')).toBeInTheDocument();
  });

  test('remove label', async () => {
    withListUnifiedResourcesSuccess();

    const { user } = renderComponent({
      props: {
        selected: [
          {
            name: 'env',
            values: ['dev'],
          },
        ],
      },
    });

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    expect(
      within(modal).getByRole('heading', { name: 'Selected Labels (1)' })
    ).toBeInTheDocument();

    const selectedSection = within(modal)
      .getByRole('heading', { name: 'Selected Labels (1)' })
      .closest('div');
    const removeLabel = within(selectedSection!).getByTestId(
      'label-action-env: dev-remove'
    );
    await user.click(removeLabel);

    const item = screen.getByTestId('cluster-item-kube-lon-dev-01.example.com');
    expect(
      within(item).getByTestId('label-action-env: dev-add')
    ).toBeInTheDocument();

    expect(
      within(modal).getByRole('heading', { name: 'Selected Labels (1)' })
    ).toBeInTheDocument();
    expect(
      within(selectedSection!).queryByText('env: dev')
    ).not.toBeInTheDocument();
    expect(within(selectedSection!).getByText('*: *')).toBeInTheDocument();
  });

  test('manual entry', async () => {
    withListUnifiedResourcesSuccess();

    const { user } = renderComponent();

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    const selectedSection = within(modal)
      .getByRole('heading', { name: 'Selected Labels (0)' })
      .closest('div');

    const nameInput = within(selectedSection!).getByLabelText('Name');
    const valueInput = within(selectedSection!).getByLabelText('Value');

    await user.type(nameInput, 'foo');
    await user.type(valueInput, 'bar{enter}');

    await user.type(nameInput, 'foo2:bar'); // colon in name
    await user.type(valueInput, 'baz{enter}');

    await user.type(nameInput, 'foo3');
    await user.type(valueInput, 'bar:baz'); // colon in value
    await user.type(nameInput, '{enter}');

    await user.type(nameInput, 'foo4');
    await user.type(valueInput, 'bar: baz{enter}'); // colon and space in value

    await user.type(nameInput, 'env');
    await user.type(valueInput, '*{enter}');

    await user.type(nameInput, 'region');
    // Extra square brackets are required to escape the initial pair
    await user.type(valueInput, '^eu-(west|east)-[[0-9]]+$');
    await user.click(
      within(selectedSection!).getByRole('button', { name: 'add label' })
    );

    expect(
      within(modal).getByRole('heading', { name: 'Selected Labels (6)' })
    ).toBeInTheDocument();

    expect(
      within(selectedSection!).getByTestId('label-action-foo: bar-remove')
    ).toBeInTheDocument();
  });

  test('done', async () => {
    withListUnifiedResourcesSuccess();

    const onChange = jest.fn();

    const { user } = renderComponent({
      props: {
        onChange,
      },
    });

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    const nameInput = within(modal).getByLabelText('Name');
    const valueInput = within(modal).getByLabelText('Value');
    await user.type(nameInput, 'foo');
    await user.type(valueInput, 'bar{enter}');

    await user.click(within(modal).getByRole('button', { name: 'Done' }));

    expect(modal).not.toBeInTheDocument();

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith([
      { name: 'foo', values: ['bar'] },
    ]);
  });

  test('cancel', async () => {
    withListUnifiedResourcesSuccess();

    const onChange = jest.fn();

    const { user } = renderComponent({
      props: {
        onChange,
      },
    });

    const edit = screen.getByRole('button', { name: 'edit' });
    await user.click(edit);

    const modal = screen.getByTestId('Modal');

    const nameInput = within(modal).getByLabelText('Name');
    const valueInput = within(modal).getByLabelText('Value');
    await user.type(nameInput, 'foo');
    await user.type(valueInput, 'bar{enter}');

    await user.click(within(modal).getByRole('button', { name: 'Cancel' }));

    expect(modal).not.toBeInTheDocument();

    expect(onChange).toHaveBeenCalledTimes(0);
  });
});

function renderComponent(opts?: {
  props?: Partial<ComponentProps<typeof KubernetesLabelsSelect>>;
  customAcl?: ReturnType<typeof makeAcl>;
  disableTracking?: boolean;
}) {
  const { props, customAcl, disableTracking } = opts ?? {};
  const { selected = [], onChange = () => {} } = props ?? {};

  const user = userEvent.setup();

  return {
    ...render(
      <KubernetesLabelsSelect selected={selected} onChange={onChange} />,
      {
        wrapper: makeWrapper({
          customAcl,
          disableTracking,
        }),
      }
    ),
    user,
  };
}

function makeWrapper(opts?: {
  customAcl?: ReturnType<typeof makeAcl>;
  disableTracking?: boolean;
}) {
  const {
    customAcl = makeAcl({
      kubeServers: {
        ...defaultAccess,
        read: true,
        list: true,
      },
    }),
    disableTracking,
  } = opts ?? {};

  const ctx = createTeleportContext({
    customAcl,
  });

  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          <ContextProvider ctx={ctx}>
            <TrackingProvider disabled={disableTracking}>
              <Validation>{children}</Validation>
            </TrackingProvider>
          </ContextProvider>
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    );
  };
}

function withListUnifiedResourcesSuccess(opts?: {
  response?: ResourcesResponse<UnifiedResource>;
}) {
  server.use(
    fetchUnifiedResourcesSuccess({
      response: opts?.response
        ? {
            ...opts.response,
            items: opts.response.agents,
          }
        : undefined,
    })
  );
}
