import { ResourceViewConfig } from 'teleport/Discover/flow';
import { DiscoverEvent } from 'teleport/services/userEvent';
import { ResourceKind } from 'teleport/Discover/Shared';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { DownloadMetadata, AddServiceProvider } from './Generic';
import { Finished } from './Finished';

import { DownloadMetadataGrafana, AddGrafanaSaml } from './Grafana';
import {
  ConfigureWorkforcePool,
  AddWorkforcePoolToTeleport,
} from './GcpWorkforce';

export const SamlApplicationResource: ResourceViewConfig = {
  kind: ResourceKind.SamlApplication,
  views(resource) {
    let configureResourceViews;
    let title;
    switch (resource.samlMeta?.preset) {
      case SamlServiceProviderPreset.Grafana:
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
        break;
      case SamlServiceProviderPreset.GcpWorkforce:
        title = 'Add SAML Application (GCP Workforce Identity Federation)';
        configureResourceViews = [
          {
            title: 'Configure Workforce Pool Provider in GCP',
            component: ConfigureWorkforcePool,
            eventName: DiscoverEvent.Started,
          },
          {
            title: 'Add GCP Workforce Pool to Teleport',
            component: AddWorkforcePoolToTeleport,
            eventName: DiscoverEvent.DeployService,
          },
        ];
        break;
      default:
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
