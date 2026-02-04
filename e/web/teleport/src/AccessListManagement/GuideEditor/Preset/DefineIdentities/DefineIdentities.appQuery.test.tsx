import { act, within } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { CreateAccessList } from 'e-teleport/AccessListManagement/CreateAccessList/CreateAccessList';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';
import cfg from 'teleport/config';
import ResourceService from 'teleport/services/resources';

import { AppIdentities } from '../role/resources/app';
import {
  fetchUnifiedResources,
  makeHandlers,
  unifiedResourcePath,
} from '../TestHelper/mocks';
import { ProviderWithQuery } from '../TestHelper/ProviderWithQuery';

const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultAccessListentitlement = cfg.entitlements.AccessLists;

const server = setupServer();
const mio = mockIntersectionObserver();

beforeAll(() => {
  server.listen();
});

let spiedUnifiedResource;
beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));

  spiedUnifiedResource = jest.spyOn(
    ResourceService.prototype,
    'fetchUnifiedResources'
  );

  cfg.isEnterprise = true;
  cfg.entitlements.AccessLists = { enabled: true, limit: 0 };
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
  cfg.isEnterprise = defaultIsEnterpriseFlag;
  cfg.entitlements.AccessLists = defaultAccessListentitlement;
});

afterAll(() => {
  server.close();
});

test('queries for app types when identities are not pre-determined', async () => {
  const user = userEvent.setup();

  const queriedAppTypes: (keyof AppIdentities)[] = [];

  let genericAppFetchCount = 0;
  server.use(
    http.get(unifiedResourcePath, ({ request }) => {
      const params = new URL(request.url).searchParams;
      const query = params.get('query') || '';

      if (query.includes('resource.spec.cloud == "AWS"')) {
        queriedAppTypes.push('aws_role_arns');
        return HttpResponse.json({
          items: [
            {
              kind: 'app',
              name: 'AwsConsoleApp',
              awsConsole: true,
            },
          ],
        });
      }
      if (query.includes('resource.spec.cloud == "Azure"')) {
        queriedAppTypes.push('azure_identities');
        return HttpResponse.json({
          items: [
            {
              kind: 'app',
              name: 'AzureApp',
              uri: 'cloud://Azure',
            },
          ],
        });
      }
      if (query.includes('resource.spec.cloud == "GCP"')) {
        queriedAppTypes.push('gcp_service_accounts');
        // Testing empty response.
        return HttpResponse.json({ items: [] });
      }
      if (query.includes('resource.sub_kind == "mcp"')) {
        queriedAppTypes.push('mcp');
        return HttpResponse.json({
          items: [
            {
              kind: 'app',
              name: 'McpApp',
              subKind: 'mcp',
            },
          ],
        });
      }

      genericAppFetchCount++;

      const requestStartKey = params.get('startKey') || '';
      const response: { items: object[]; startKey?: string } = {
        items: [],
      };

      if (!requestStartKey) {
        response.items = [
          {
            kind: 'app',
            name: 'GenericApp',
            labels: [{ name: 'env', value: 'test' }],
          },
        ];
      }

      // After the first fetch, always return a startKey with an empty result
      // to stop triggering the infinite scrolling from new data.
      if (genericAppFetchCount >= 2) {
        response.startKey = 'next-page-key-triggers-the-query';
      }

      return HttpResponse.json(response);
    })
  );

  render(
    <ProviderWithQuery>
      <AccessGraphDemoProvider>
        <CreateAccessList />
      </AccessGraphDemoProvider>
    </ProviderWithQuery>
  );

  // Start the guide
  await screen.findByText(/Select the type of Access List/i);
  await user.click(screen.getByText(/temporary access/i));

  await screen.findByText(/define application access/i);
  act(mio.enterAll); // trigger the isIntersecting of IntersectionObserver
  await screen.findByText(/no access defined/i);

  // Select any label
  let targetRow = screen.getByTestId('GenericApp');
  await user.click(within(targetRow).getByTitle(/env: test/i));
  act(mio.enterAll);
  await screen.findByText('GenericApp');
  spiedUnifiedResource.mockClear();

  // Go to identities step
  // Querying for apps will trigger since in the previou step
  // the latest app query resulted in a "startKey" indicating
  // there are more pages and no identities were marked seen
  await user.click(screen.getByRole('button', { name: /next/i }));
  await screen.findByText(/application identities/i);

  // Verify all types were queried
  expect(queriedAppTypes).toEqual([
    'aws_role_arns',
    'azure_identities',
    'gcp_service_accounts',
    'mcp',
  ]);
  expect(spiedUnifiedResource).toHaveBeenCalledTimes(4);

  // Verify that the correct identity fields were rendered
  expect(screen.getByText(/aws/i)).toBeInTheDocument();
  expect(screen.getByText(/mcp/i)).toBeInTheDocument();
  expect(screen.getByText(/azure/i)).toBeInTheDocument();

  // Empty b/c query for gcp returned no result
  expect(screen.queryByText(/gcp/i)).not.toBeInTheDocument();

  expect(screen.queryAllByRole('textbox')).toHaveLength(3);
});
