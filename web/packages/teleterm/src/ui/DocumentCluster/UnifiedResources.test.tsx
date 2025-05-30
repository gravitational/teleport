/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { act } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import {
  createRef,
  FC,
  forwardRef,
  Profiler,
  PropsWithChildren,
  useImperativeHandle,
} from 'react';

import { Providers, render, screen } from 'design/utils/testing';
import { ShowResources } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import {
  AvailableResourceMode,
  DefaultTab,
  LabelsViewMode,
  ViewMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeRootCluster,
  makeServer,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { ConnectMyComputerContextProvider } from 'teleterm/ui/ConnectMyComputer';
import {
  ResourcesContextProvider,
  useResourcesContext,
} from 'teleterm/ui/DocumentCluster/resourcesContext';
import { UnifiedResources } from 'teleterm/ui/DocumentCluster/UnifiedResources';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import * as uri from 'teleterm/ui/uri';

const mio = mockIntersectionObserver();

test.each([
  {
    name: 'fetches only available resources if cluster does not support access requests',
    conditions: {
      isClusterSupportingAccessRequests: false,
      showResources: ShowResources.REQUESTABLE,
      availableResourceModePreference: AvailableResourceMode.ALL,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: false,
    },
  },
  {
    name: 'fetches all resources if cluster allows listing all and user preferences says all',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.REQUESTABLE,
      availableResourceModePreference: AvailableResourceMode.ALL,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: true,
    },
  },
  {
    name: 'fetches all resources if cluster allows listing all and user preferences says none',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.REQUESTABLE,
      availableResourceModePreference: AvailableResourceMode.ALL,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: true,
    },
  },
  {
    name: 'fetches accessible resources if cluster allows listing all and user preferences says accessible',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.REQUESTABLE,
      availableResourceModePreference: AvailableResourceMode.ACCESSIBLE,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: false,
    },
  },
  {
    name: 'fetches requestable resources if cluster allows listing all and user preferences says requestable',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.REQUESTABLE,
      availableResourceModePreference: AvailableResourceMode.REQUESTABLE,
    },
    expect: {
      searchAsRoles: true,
      includeRequestable: false,
    },
  },
  {
    name: 'fetches only accessible resources if cluster does not allow listing all',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.ACCESSIBLE_ONLY,
      availableResourceModePreference: AvailableResourceMode.UNSPECIFIED,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: false,
    },
  },
  {
    name: 'fetches only accessible resources if cluster does not allow listing all and user preferences says accessible',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.ACCESSIBLE_ONLY,
      availableResourceModePreference: AvailableResourceMode.ALL,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: false,
    },
  },
  {
    name: 'fetches only requestable resources if cluster does not allow listing all and user preferences says requestable',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.ACCESSIBLE_ONLY,
      availableResourceModePreference: AvailableResourceMode.REQUESTABLE,
    },
    expect: {
      searchAsRoles: true,
      includeRequestable: false,
    },
  },
  {
    name: 'fetches only accessible resources if cluster does not allow listing all but user preferences says all',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.ACCESSIBLE_ONLY,
      availableResourceModePreference: AvailableResourceMode.ALL,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: false,
    },
  },
  {
    name: 'fetches only accessible resources if cluster does not allow listing all but user preferences says none',
    conditions: {
      isClusterSupportingAccessRequests: true,
      showResources: ShowResources.ACCESSIBLE_ONLY,
      availableResourceModePreference: AvailableResourceMode.NONE,
    },
    expect: {
      searchAsRoles: false,
      includeRequestable: false,
    },
  },
])('$name', async testCase => {
  const doc = makeDocumentCluster();

  const appContext = new MockAppContext({ platform: 'darwin' });
  appContext.addRootClusterWithDoc(
    makeRootCluster({
      uri: doc.clusterUri,
      features: {
        advancedAccessWorkflows:
          testCase.conditions.isClusterSupportingAccessRequests,
        isUsageBasedBilling: false,
      },
      showResources: testCase.conditions.showResources,
    }),
    doc
  );
  appContext.workspacesService.setState(draftState => {
    draftState.workspaces[doc.clusterUri].unifiedResourcePreferences = {
      defaultTab: DefaultTab.ALL,
      viewMode: ViewMode.CARD,
      labelsViewMode: LabelsViewMode.COLLAPSED,
      availableResourceMode:
        testCase.conditions.availableResourceModePreference,
    };
  });

  jest.spyOn(appContext.tshd, 'getUserPreferences').mockResolvedValue(
    new MockedUnaryCall({
      userPreferences: {
        unifiedResourcePreferences: {
          defaultTab: DefaultTab.ALL,
          viewMode: ViewMode.CARD,
          labelsViewMode: LabelsViewMode.COLLAPSED,
          availableResourceMode:
            testCase.conditions.availableResourceModePreference,
        },
      },
    })
  );

  jest
    .spyOn(appContext.resourcesService, 'listUnifiedResources')
    .mockResolvedValue({
      resources: [],
      nextKey: '',
    });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <ConnectMyComputerContextProvider rootClusterUri={doc.clusterUri}>
            <UnifiedResources
              clusterUri={doc.clusterUri}
              docUri={doc.uri}
              queryParams={doc.queryParams}
            />
          </ConnectMyComputerContextProvider>
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  act(mio.enterAll);

  await expect(
    screen.findByText('Add your first resource to Teleport')
  ).resolves.toBeInTheDocument();

  expect(appContext.resourcesService.listUnifiedResources).toHaveBeenCalledWith(
    {
      clusterUri: rootClusterUri,
      includeRequestable: testCase.expect.includeRequestable,
      kinds: [],
      limit: 48,
      pinnedOnly: false,
      query: '',
      search: '',
      searchAsRoles: testCase.expect.searchAsRoles,
      sortBy: {
        field: 'name',
        isDesc: false,
      },
      startKey: '',
    },
    new AbortController().signal
  );
});

test.each([
  {
    name: 'refreshes resources when the document cluster URI matches the requested cluster URI',
    conditions: {
      documentClusterUri: '/clusters/teleport-local',
    },
    expect: {
      resourcesRefreshed: true,
    },
  },
  {
    name: 'refreshes resources when the document cluster URI is a leaf of the requested cluster URI',
    conditions: {
      documentClusterUri: '/clusters/teleport-local/leaves/leaf',
    },
    expect: {
      resourcesRefreshed: true,
    },
  },
])('$name', async testCase => {
  const doc = makeDocumentCluster({
    clusterUri: testCase.conditions.documentClusterUri,
  });
  const rootCluster = makeRootCluster({
    uri: uri.routing.ensureRootClusterUri(doc.clusterUri),
  });
  const serverResource = makeServer();
  const appContext = new MockAppContext();
  appContext.addRootClusterWithDoc(rootCluster, doc);

  jest
    .spyOn(appContext.resourcesService, 'listUnifiedResources')
    .mockResolvedValue({
      resources: [
        {
          kind: 'server',
          resource: serverResource,
          requiresRequest: false,
        },
      ],
      nextKey: '',
    });

  const ref = createRef<{
    requestResourcesRefresh: () => void;
  }>();

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <ResourcesContextProvider>
          <ConnectMyComputerContextProvider rootClusterUri={rootCluster.uri}>
            <Refresher ref={ref} rootClusterUri={rootCluster.uri} />
            <UnifiedResources
              clusterUri={doc.clusterUri}
              docUri={doc.uri}
              queryParams={doc.queryParams}
            />
          </ConnectMyComputerContextProvider>
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  act(mio.enterAll);

  // Wait for resources to render.
  await expect(
    screen.findByText(serverResource.hostname)
  ).resolves.toBeInTheDocument();
  expect(
    appContext.resourcesService.listUnifiedResources
  ).toHaveBeenCalledTimes(1);

  act(() => ref.current.requestResourcesRefresh());

  // Wait for resources to (potentially) re-render.
  await expect(
    screen.findByText(serverResource.hostname)
  ).resolves.toBeInTheDocument();
  expect(
    appContext.resourcesService.listUnifiedResources
    // When resources are refreshed, we have two calls to the API.
  ).toHaveBeenCalledTimes(testCase.expect.resourcesRefreshed ? 2 : 1);
});

const Refresher = forwardRef<
  {
    requestResourcesRefresh: () => void;
  },
  {
    rootClusterUri: uri.RootClusterUri;
  }
>((props, ref) => {
  const resourcesContext = useResourcesContext(props.rootClusterUri);
  useImperativeHandle(ref, () => ({
    requestResourcesRefresh: resourcesContext.requestResourcesRefresh,
  }));
  return null;
});

it('passes props with stable identity to <Resources>', async () => {
  const rootCluster = makeRootCluster();
  const doc = makeDocumentCluster({
    clusterUri: rootCluster.uri,
  });
  const serverResource = makeServer();
  const appContext = new MockAppContext();
  appContext.addRootClusterWithDoc(rootCluster, doc);

  jest
    .spyOn(appContext.resourcesService, 'listUnifiedResources')
    .mockResolvedValue({
      resources: [
        {
          kind: 'server',
          resource: serverResource,
          requiresRequest: false,
        },
      ],
      nextKey: '',
    });

  let renderCount = 0;
  const onRender = () => (renderCount += 1);

  const Wrapper: FC<PropsWithChildren> = ({ children }) => (
    <Providers>
      <MockAppContextProvider appContext={appContext}>
        <MockWorkspaceContextProvider>
          <ResourcesContextProvider>
            <ConnectMyComputerContextProvider rootClusterUri={doc.clusterUri}>
              {children}
            </ConnectMyComputerContextProvider>
          </ResourcesContextProvider>
        </MockWorkspaceContextProvider>
      </MockAppContextProvider>
    </Providers>
  );

  const Component = () => (
    <Profiler id="unifiedResources" onRender={onRender}>
      <UnifiedResources
        clusterUri={doc.clusterUri}
        docUri={doc.uri}
        queryParams={doc.queryParams}
      />
    </Profiler>
  );

  const { rerender } = render(<Component />, { wrapper: Wrapper });
  act(mio.enterAll);
  // Wait for resources to render.
  await expect(
    screen.findByText(serverResource.hostname)
  ).resolves.toBeInTheDocument();
  // Disabled because of a false positive.
  // eslint-disable-next-line testing-library/render-result-naming-convention
  const renderCountBeforeRerender = renderCount;

  rerender(<Component />);
  await expect(
    screen.findByText(serverResource.hostname)
  ).resolves.toBeInTheDocument();
  const renderCountDelta = renderCount - renderCountBeforeRerender;

  // When <Resources> is properly memoized and all params passed to it have stable identity,
  // rerendering the outer <UnifiedResources> with no prop changes should cause only one render
  // within the tree.
  expect(renderCountDelta).toEqual(1);
});
