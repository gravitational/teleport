// This is the loader for the access graph component
//
// It will download the access graph library from the CDN and then render the component

import React, { ComponentType, lazy, Suspense } from 'react';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { useFeatures } from 'teleport/FeaturesContext';
import { getFirstRouteForCategory } from 'teleport/Navigation/Navigation';
import { NavigationCategory } from 'teleport/Navigation/categories';

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

export function AccessGraph() {
  const { clusterId } = useStickyClusterId();

  const features = useFeatures();
  const backUrl = getFirstRouteForCategory(
    features,
    NavigationCategory.Management
  );

  return (
    <Suspense fallback={<AccessGraphLoading />}>
      <Graph
        clusterId={clusterId}
        awsOnboardingEnabled={true}
        urlNavigationEnabled={true}
        cfg={cfg}
        backUrl={backUrl}
        s3BucketOptional={true}
      />
    </Suspense>
  );
}
