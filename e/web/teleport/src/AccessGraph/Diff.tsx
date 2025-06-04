// This is the loader for the access graph diff component
//
// It will download the access graph library from the CDN and then render the component

import { ComponentType, lazy, Suspense } from 'react';

import { AccessGraphLoading } from 'e-teleport/AccessGraph/AccessGraphLoading';
import { loadAccessGraph } from 'e-teleport/AccessGraph/loader';

interface GenericNode {
  id: string;
  kind: string;
  name: string;
  sub_kind: string;
  source: string;
  origin_type: string;
}

interface Edge {
  id?: string;
  from: string;
  to: string;
  edge_type: string;
  properties?: {
    [key: string]: string;
  };
}

interface Operation {
  op: 'add' | 'remove' | 'replace' | 'move' | 'copy' | 'test';
  path?: string;
  from?: string;
  value?: Record<string, any>;
}

interface GenericNodesList {
  nodes: GenericNode[];
  edges: Edge[];
}

export interface AccessPathDiff {
  change_id: string;
  base: GenericNodesList;
  diff: Operation[];
}

interface AccessGraphDiffProps {
  diff: AccessPathDiff;
  loading: boolean;
}

declare global {
  interface Window {
    AccessGraphDiff: {
      Diff: ComponentType<AccessGraphDiffProps>;
    };
  }
}

const Diff = lazy(() =>
  loadAccessGraph<AccessGraphDiffProps>(
    'access-graph-diff-react-19.umd.js',
    () => window.AccessGraphDiff.Diff
  )
);

export function AccessGraphDiff(props: AccessGraphDiffProps) {
  return (
    <Suspense fallback={<AccessGraphLoading />}>
      <Diff {...props} />
    </Suspense>
  );
}
