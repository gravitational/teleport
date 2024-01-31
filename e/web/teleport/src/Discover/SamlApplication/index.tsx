import { ResourceViewConfig } from 'teleport/Discover/flow';
import { DiscoverEvent } from 'teleport/services/userEvent';
import { ResourceKind } from 'teleport/Discover/Shared';

import { DownloadMetadata, AddServiceProvider } from './SamlApp';
import { Finished } from './Finished';

import { DownloadMetadataGrafana, AddGrafanaSaml } from './SamlAppGrafana';

export const SamlApplicationResource: ResourceViewConfig = {
  kind: ResourceKind.SamlApplication,
  views(resource) {
    let configureResourceViews;
    let title;
    if (resource.name.toLowerCase().includes('grafana')) {
      title = 'Add SAML Application (Grafana)';
      configureResourceViews = [
        {
          title: "Configure Grafana with Teleport's IdP Metadata",
          component: DownloadMetadataGrafana,
          eventName: DiscoverEvent.Started,
        },
        {
          title: "Add Grafana's Service Provider to Teleport",
          component: AddGrafanaSaml,
          eventName: DiscoverEvent.DeployService,
        },
      ];
    } else {
      title = 'Add SAML Application';
      configureResourceViews = [
        {
          title: "Configure Service Provider with Teleport's IdP Metadata",
          component: DownloadMetadata,
          eventName: DiscoverEvent.Started,
        },
        {
          title: 'Add Service Provider to Teleport',
          component: AddServiceProvider,
          eventName: DiscoverEvent.DeployService,
        },
      ];
    }
    return [
      { title, views: configureResourceViews },
      {
        title: 'Finished',
        component: Finished,
        hide: true,
        eventName: DiscoverEvent.Completed,
      },
    ];
  },
};
