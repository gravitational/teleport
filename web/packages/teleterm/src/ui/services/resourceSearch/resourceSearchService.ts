import { useStore, Store } from 'shared/libs/stores';

import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import { ClusterUri } from 'teleterm/ui/uri';

export class ResourceSearchService extends Store<{
  connector?: SearchConnector;
}> {
  state: { connector?: SearchConnector } = {};

  setConnector(connector: SearchConnector): void {
    this.setState({ connector });
  }

  getConnector() {
    return this.state.connector;
  }

  useState() {
    return useStore(this).state;
  }
}

export class SearchConnector extends ImmutableStore<{
  search: string;
  kinds: ('db' | 'kube_cluster' | 'node')[];
  isAdvancedSearchEnabled: boolean;
}> {
  update(
    searchBarState: Partial<{
      search: string;
      kinds: ('db' | 'kube_cluster' | 'node')[];
      isAdvancedSearchEnabled: boolean;
    }>
  ): void {
    this.setState(prev => ({ ...prev, ...searchBarState }));
  }

  constructor(
    public readonly clusterUri: ClusterUri,
    initValues: {
      search: string;
      kinds: ('db' | 'kube_cluster' | 'node')[];
      isAdvancedSearchEnabled: boolean;
    }
  ) {
    super();
    this.setState(() => initValues);
  }

  getState() {
    return this.state;
  }
}
