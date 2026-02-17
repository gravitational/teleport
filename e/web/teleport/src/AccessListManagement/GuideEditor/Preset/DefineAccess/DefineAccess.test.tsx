/* eslint-disable jest/no-conditional-expect */
import { act, within } from '@testing-library/react';
import { UserEvent } from '@testing-library/user-event';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import selectEvent from 'react-select-event';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import ResourceService from 'teleport/services/resources';

import { definableResourceAccessFields } from '../role/listaccess';
import {
  fetchUnifiedResources,
  makeHandlers,
  withCustomError,
} from '../TestHelper/mocks';
import { ProviderWithQuery } from '../TestHelper/ProviderWithQuery';
import { DefineAccess } from './DefineAccess';

const mio = mockIntersectionObserver();

enableMswServer();

let spiedUnifiedResource;
beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
  spiedUnifiedResource = jest.spyOn(
    ResourceService.prototype,
    'fetchUnifiedResources'
  );
});

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.clearAllMocks();
});

/**
 * Testing for AWS IC is in its own file AwsIcSection.test
 */
describe('DefineAccess', () => {
  test(`empty state when there are no resources in cluster`, async () => {
    const user = userEvent.setup();
    server.use(...makeHandlers([fetchUnifiedResources('get', [])]));

    render(
      <ProviderWithQuery>
        <DefineAccess />
      </ProviderWithQuery>
    );

    // resource application is the default tab
    await screen.findByText(/define application access/i);
    act(mio.enterAll); // trigger the isIntersecting of IntersectionObserver

    expect(spiedUnifiedResource).toHaveBeenCalledTimes(2);

    // Initial call when the provider initializes
    // this data is used for the AWS IC tab.
    expect(spiedUnifiedResource).toHaveBeenNthCalledWith(
      1,
      expect.anything(),
      {
        kinds: ['app'],
        limit: 200,
        query: 'resource.sub_kind == "aws_ic_account"',
        startKey: '',
      },
      expect.anything()
    );

    // The call when rendering the application tab.
    expect(spiedUnifiedResource).toHaveBeenNthCalledWith(
      2,
      expect.anything(),
      {
        kinds: ['app'],
        limit: 48,
        // AWS IC application types have their own "tab"
        query: 'labels["teleport.dev/origin"] != "aws-identity-center"',
        search: undefined,
        sort: { dir: 'ASC', fieldName: 'name' },
        startKey: '',
      },
      expect.anything()
    );

    await screen.findByText(/no application found/i);
    expect(
      screen.getByText(/not have permission to access any applications/i)
    ).toBeInTheDocument();
    expect(screen.queryByText(/click on labels/i)).not.toBeInTheDocument();

    // test other tabs
    for (let i = 0; i < definableResourceAccessFields.length; i++) {
      spiedUnifiedResource.mockClear();

      const field = definableResourceAccessFields[i];
      if (field === 'app_labels') {
        continue; // skip testing as it was just tested above.
      }

      await user.click(screen.getByTestId(field));

      switch (field) {
        case 'awsIc':
          await screen.findByText(/no aws identity center found/i);
          expect(
            screen.getByText(
              /not have permission to access any aws accounts and permission sets/i
            )
          ).toBeInTheDocument();
          expect(
            screen.queryByRole('button', { name: /make a new selection/i })
          ).not.toBeInTheDocument();

          // AWS IC is pre-fetched when provider is loaded.
          expect(spiedUnifiedResource).toHaveBeenCalledTimes(0);
          break;

        case 'github_permissions':
          await screen.findByText(/no git server found/i);
          expect(
            screen.getByText(/not have permission to access any git servers/i)
          ).toBeInTheDocument();
          expect(
            screen.queryByPlaceholderText(/click to select/i)
          ).not.toBeInTheDocument();

          expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
          expect(spiedUnifiedResource).toHaveBeenNthCalledWith(
            1,
            expect.anything(),
            {
              kinds: ['git_server'],
              limit: 50,
              search: undefined,
            }
          );
          break;

        case 'db_labels':
          await screen.findByText(/define database access/i);
          act(mio.enterAll);
          await screen.findByText(/no database found/i);
          expect(
            screen.getByText(/not have permission to access any databases/i)
          ).toBeInTheDocument();
          expect(
            screen.queryByPlaceholderText(/type a label/i)
          ).not.toBeInTheDocument();

          expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
          expect(spiedUnifiedResource).toHaveBeenCalledWith(
            expect.anything(),
            {
              kinds: ['db'],
              limit: 48,
              query: '',
              search: undefined,
              sort: { dir: 'ASC', fieldName: 'name' },
              startKey: '',
            },
            expect.anything()
          );
          break;

        case 'kubernetes_labels':
          await screen.findByText(/define kubernetes cluster access/i);
          act(mio.enterAll);
          await screen.findByText(/no kubernetes cluster found/i);
          expect(
            screen.getByText(
              /not have permission to access any kubernetes clusters/i
            )
          ).toBeInTheDocument();
          expect(
            screen.queryByPlaceholderText(/type a label/i)
          ).not.toBeInTheDocument();

          expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
          expect(spiedUnifiedResource).toHaveBeenCalledWith(
            expect.anything(),
            {
              kinds: ['kube_cluster'],
              limit: 48,
              query: '',
              search: undefined,
              sort: { dir: 'ASC', fieldName: 'name' },
              startKey: '',
            },
            expect.anything()
          );
          break;

        case 'node_labels':
          await screen.findByText(/define server access/i);
          act(mio.enterAll);
          await screen.findByText(/no server found/i);
          expect(
            screen.getByText(/not have permission to access any servers/i)
          ).toBeInTheDocument();
          expect(
            screen.queryByPlaceholderText(/type a label/i)
          ).not.toBeInTheDocument();

          expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
          expect(spiedUnifiedResource).toHaveBeenCalledWith(
            expect.anything(),
            {
              kinds: ['node'],
              limit: 48,
              query: '',
              search: undefined,
              sort: { dir: 'ASC', fieldName: 'name' },
              startKey: '',
            },
            expect.anything()
          );
          break;

        case 'windows_desktop_labels':
          await screen.findByText(/define windows desktop access/i);
          act(mio.enterAll);
          await screen.findByText(/no windows desktop found/i);
          expect(
            screen.getByText(
              /not have permission to access any windows desktops/i
            )
          ).toBeInTheDocument();
          expect(
            screen.queryByPlaceholderText(/type a label/i)
          ).not.toBeInTheDocument();

          expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
          expect(spiedUnifiedResource).toHaveBeenCalledWith(
            expect.anything(),
            {
              kinds: ['windows_desktop'],
              limit: 48,
              query: '',
              search: undefined,
              sort: { dir: 'ASC', fieldName: 'name' },
              startKey: '',
            },
            expect.anything()
          );
          break;

        default:
          field satisfies never;
      }
      expect(screen.queryByText(/no access defined/i)).not.toBeInTheDocument();
    }
  });

  test(`empty state when there are resources in cluster and no selections are made`, async () => {
    async function testGoingNextWithNoSelectionRendersDialog(user: UserEvent) {
      await user.click(screen.getByRole('button', { name: /next/i }));
      await screen.findByText(/no resource access is defined/i);
      await user.click(screen.getByRole('button', { name: /cancel/i }));
    }

    const user = userEvent.setup();

    render(
      <ProviderWithQuery>
        <DefineAccess />
      </ProviderWithQuery>
    );

    // resource application is the default tab
    await screen.findByText(/define application access/i);
    act(mio.enterAll); // trigger the isIntersecting of IntersectionObserver

    await screen.findByText(/no access defined/i);
    expect(
      screen.getByText(/type a label and press enter/i)
    ).toBeInTheDocument();
    expect(screen.getByText('AppTestRow')).toBeInTheDocument();
    expect(screen.queryByText(/no application found/i)).not.toBeInTheDocument();

    await testGoingNextWithNoSelectionRendersDialog(user);

    // test other tabs
    for (let i = 0; i < definableResourceAccessFields.length; i++) {
      const field = definableResourceAccessFields[i];
      if (field === 'app_labels') {
        continue; // skip testing as it was just tested above.
      }

      await user.click(screen.getByTestId(field));

      await withCustomError(field, async () => {
        expect(
          await screen.findByText(/define .* access/i)
        ).toBeInTheDocument();
        act(mio.enterAll);
        expect(
          await screen.findByText(/no access defined/i)
        ).toBeInTheDocument();
      });

      switch (field) {
        case 'awsIc':
          expect(
            screen.getByRole('button', { name: /make a new selection/i })
          ).toBeInTheDocument();
          expect(
            screen.queryByText(/no aws identity center found/i)
          ).not.toBeInTheDocument();
          break;

        case 'github_permissions':
          expect(
            screen.queryByTestId('unified-resource-table')
          ).not.toBeInTheDocument();

          expect(
            screen.queryByText(/no git server found/i)
          ).not.toBeInTheDocument();

          expect(
            screen.getByText(/click to select or search for github org/i)
          ).toBeInTheDocument();
          break;

        case 'db_labels':
          expect(screen.getByText('DbTestRow')).toBeInTheDocument();
          break;

        case 'kubernetes_labels':
          expect(screen.getByText('KubeClusterTestRow')).toBeInTheDocument();
          break;

        case 'node_labels':
          expect(screen.getByText('NodeTestRow')).toBeInTheDocument();
          break;

        case 'windows_desktop_labels':
          expect(screen.getByText('WindowsTestRow')).toBeInTheDocument();
          break;

        default:
          field satisfies never;
      }
      await testGoingNextWithNoSelectionRendersDialog(user);
    }
  });

  test(`selecting github orgs`, async () => {
    const user = userEvent.setup();

    render(
      <ProviderWithQuery>
        <DefineAccess />
      </ProviderWithQuery>
    );

    await screen.findByText(/define application access/i);
    await user.click(screen.getByTestId('github_permissions'));

    await screen.findByText(/no access defined/i);
    spiedUnifiedResource.mockClear();

    // Table isn't rendered until user selects wildcard.
    expect(
      screen.queryByTestId('unified-resource-table')
    ).not.toBeInTheDocument();

    expect(
      screen.getByText(/select which github organizations are allowed/i)
    ).toBeInTheDocument();

    /**
     * Select a org from dropdown
     */
    const reactSelectInput = screen.getByRole('combobox');

    await selectEvent.select(reactSelectInput, 'GitServerTestRow');

    const inputWrapper = screen.getByTestId('git-server-dropdown');
    expect(
      within(inputWrapper).getByText(/GitServerTestRow/i)
    ).toBeInTheDocument();
    expect(screen.queryByText(/no access defined/i)).not.toBeInTheDocument();
    expect(
      screen.queryByTestId('unified-resource-table')
    ).not.toBeInTheDocument();

    expect(spiedUnifiedResource).toHaveBeenCalledTimes(0);

    /**
     * Selecting wildcard should clear previous selections
     * and render the unified resource table
     */
    await user.click(reactSelectInput);
    await user.click(screen.getByText(/use wildcard/i));

    const unifiedResourceTable = await screen.findByTestId(
      /unified-resource-table/i
    );
    act(mio.enterAll);

    await within(unifiedResourceTable).findByText(/GitServerTestRow/i);
    expect(within(inputWrapper).getByText('*')).toBeInTheDocument();
    expect(
      within(inputWrapper).queryByText(/GitServerTestRow/i)
    ).not.toBeInTheDocument();

    expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
    expect(spiedUnifiedResource).toHaveBeenCalledWith(
      expect.anything(),
      {
        kinds: ['git_server'],
        limit: 48,
        query: undefined,
        startKey: '',
        sort: { dir: 'ASC', fieldName: 'name' },
        search: undefined,
      },
      expect.anything()
    );
    spiedUnifiedResource.mockClear();

    /**
     * Removing all selections renders empty state.
     */
    await user.click(
      within(inputWrapper).getByRole('button', {
        name: 'Remove *',
      })
    );
    await screen.findByText(/no access defined/i);

    expect(
      screen.queryByTestId('unified-resource-table')
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/select which github organizations are allowed/i)
    ).toBeInTheDocument();

    expect(spiedUnifiedResource).toHaveBeenCalledTimes(0);
  });
});
