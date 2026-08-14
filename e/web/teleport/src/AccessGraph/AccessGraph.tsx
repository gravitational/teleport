// This is the loader for the access graph component
//
// It will download the access graph library from the CDN and then render the component

import { Location } from 'history';
import { ComponentType, lazy, Suspense, useCallback, useEffect } from 'react';
import { ErrorBoundary } from 'react-error-boundary';
import { useLocation, useNavigate } from 'react-router';

import { Flex } from 'design';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import {
  AccessGraphLoadingError,
  AccessGraphSetupError,
} from 'e-teleport/AccessGraph/AccessGraphError';
import { AccessGraphLoading } from 'e-teleport/AccessGraph/AccessGraphLoading';
import {
  ACCESS_GRAPH_JS_FILE,
  loadAccessGraph,
} from 'e-teleport/AccessGraph/loader';
import cfg, { EnterpriseConfig } from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import { storageService } from 'teleport/services/storageService';
import { getCurrentTheme } from 'teleport/ThemeProvider';
import { useUser } from 'teleport/User/UserContext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { EmptyState } from './EmptyState';

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
    ACCESS_GRAPH_JS_FILE,
    () => window.AccessGraphLib.AccessGraph
  )
);

function locationIsEqual(a: Location, b: Location) {
  return a.pathname === b.pathname && a.search === b.search;
}

export function AccessGraph() {
  const ctx = useTeleportE();
  const hasAccess =
    ctx.storeUser.getAccessGraphAccess().list &&
    storageService.getAccessGraphEnabled();
  const { clusterId } = useStickyClusterId();

  const backUrl = cfg.oss.getUnifiedResourcesRoute(cfg.oss.proxyCluster);

  const { preferences } = useUser();

  const theme = getCurrentTheme(preferences.theme);

  const navigate = useNavigate();
  const location = useLocation();

  // Access Graph uses its own createBrowserRouter instance. In React Router 7, pushState
  // detection is unreliable when multiple router instances coexist.
  // Dispatch a popstate event so Access Graph's router picks up URL changes triggered by
  // Teleport's navigation.
  // TODO(ryan): Pass through the router instance to Access Graph and remove this workaround.
  useEffect(() => {
    window.dispatchEvent(
      new PopStateEvent('popstate', { state: window.history.state })
    );
  }, [location.pathname, location.search]);

  const onRouteChange = useCallback(
    (next: Location) => {
      if (!locationIsEqual(location, next)) {
        // As Access Graph has its own router, any navigation changes within
        // Access Graph do not trigger a change in Teleport's router.
        // We replace the current history entry with the new location so that
        // Teleport's router is aware of the change (and the navigation buttons
        // have the correct active state).
        navigate(next, { replace: true });
      }
    },
    [navigate, location]
  );

  if (!hasAccess) {
    if (cfg.oss.entitlements.AccessGraph.enabled) {
      return (
        <AccessGraphSetupError
          isConfigured={cfg.oss.identitySecurity.accessGraphConfigSet}
        />
      );
    }

    return <EmptyState />;
  }

  return (
    <ErrorBoundary FallbackComponent={AccessGraphLoadingError}>
      <Suspense fallback={<AccessGraphLoading />}>
        <Flex
          width="100%"
          height="100%"
          overflowY="auto"
          flexDirection="column"
        >
          <Graph
            key={theme}
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
    </ErrorBoundary>
  );
}
