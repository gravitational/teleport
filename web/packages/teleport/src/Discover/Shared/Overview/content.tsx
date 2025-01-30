import { Overview } from 'teleport/Discover/Shared/Overview/types';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

export const content: { [key in DiscoverGuideId]?: Overview } = {
  [DiscoverGuideId.Kubernetes]: {
    OverviewContent: () => (
      <ul>
        <li>
          This guide uses Helm to install the Teleport agent into a cluster, and
          by default turns on auto-discovery of all apps in the cluster.
        </li>
      </ul>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>Egress from your Kubernetes cluster to Teleport.</li>
        <li>Helm installed on your local machine.</li>
        <li>Kubernetes API access to install the Helm chart.</li>
      </ul>
    ),
  },
};
