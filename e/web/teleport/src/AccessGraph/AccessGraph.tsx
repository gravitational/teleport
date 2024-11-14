// This is the loader for the access graph component
//
// It will download the access graph library from the CDN and then render the component

import React, { ComponentType, lazy, Suspense, useCallback } from 'react';
import { useHistory, useLocation } from 'react-router';
import { Location } from 'history';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { useFeatures } from 'teleport/FeaturesContext';
import { getFirstRouteForCategory } from 'teleport/Navigation/Navigation';
import { NavigationCategory } from 'teleport/Navigation/categories';

import { getCurrentTheme } from 'teleport/ThemeProvider';

import { useUser } from 'teleport/User/UserContext';

import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import { Flex } from 'design';

import cfg, { EnterpriseConfig } from 'e-teleport/config';
import { AccessGraphLoading } from 'e-teleport/AccessGraph/AccessGraphLoading';
import { loadAccessGraph } from 'e-teleport/AccessGraph/loader';

interface AccessGraphProps {
  clusterId: string;
  awsOnboardingEnabled: boolean;
  urlNavigationEnabled: boolean;
  cfg: EnterpriseConfig;
  backUrl: string;
  s3BucketOptional: boolean;
  theme: Theme;

  onRouteChange(location: Location): void;
}

declare global {
  interface Window {
    AccessGraphLib: {
      AccessGraph: ComponentType<AccessGraphProps>;
    };
  }
}

const Graph = lazy(() =>
  loadAccessGraph<AccessGraphProps>(
    'access-graph.umd.js',
    () => window.AccessGraphLib.AccessGraph
  )
);

function locationIsEqual(a: Location, b: Location) {
  return a.pathname === b.pathname && a.search === b.search;
}

export function AccessGraph() {
  const { clusterId } = useStickyClusterId();

  const features = useFeatures();
  const backUrl = getFirstRouteForCategory(
    features,
    NavigationCategory.Management
  );

  const { preferences } = useUser();

  const theme = getCurrentTheme(preferences.theme);

  const history = useHistory();
  const location = useLocation();

  const onRouteChange = useCallback(
    (next: Location) => {
      if (!locationIsEqual(location, next)) {
        // As Access Graph has its own router, any navigation changes within
        // Access Graph do not trigger a change in Teleport's router.
        // We replace the current history entry with the new location so that
        // Teleport's router is aware of the change (and the navigation buttons
        // have the correct active state).
        history.replace(next);
      }
    },
    [history, location]
  );

  return (
    <Suspense fallback={<AccessGraphLoading />}>
      <Flex
        data-scrollbar="default"
        width="100%"
        height="100%"
        overflowY="auto"
        flexDirection="column"
      >
        <Graph
          clusterId={clusterId}
          awsOnboardingEnabled={true}
          urlNavigationEnabled={true}
          cfg={cfg}
          backUrl={backUrl}
          s3BucketOptional={true}
          onRouteChange={onRouteChange}
          theme={theme}
        />
      </Flex>
    </Suspense>
  );
}
