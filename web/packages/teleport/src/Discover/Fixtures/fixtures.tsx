/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { LocationDescriptor } from 'history';
import React, { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { DiscoverBox, ResourceKind } from 'teleport/Discover/Shared';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import {
  AgentMeta,
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { Acl, AuthType } from 'teleport/services/user';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';
import TeleportContext from 'teleport/teleportContext';
import { ThemeProvider } from 'teleport/ThemeProvider';
import { TeleportFeature } from 'teleport/types';

import {
  APPLICATIONS,
  KUBERNETES,
  SAML_APPLICATIONS,
  SelectResourceSpec,
  SERVERS,
} from '../SelectResource/resources';

export const RequiredDiscoverProviders: React.FC<
  PropsWithChildren<{
    agentMeta: AgentMeta;
    resourceSpec: SelectResourceSpec;
    interval?: number;
    customAcl?: Acl;
    authType?: AuthType;
    teleportCtx?: TeleportContext;
    discoverCtx?: DiscoverContextState;
    features?: TeleportFeature[];
    initialEntries?: LocationDescriptor<unknown>[];
  }>
> = props => {
  let ctx = createTeleportContext({ customAcl: props.customAcl });
  if (props.teleportCtx) {
    ctx = props.teleportCtx;
  }
  if (props.authType) {
    ctx.storeUser.state.authType = props.authType;
  }
  let discoverCtx;
  if (props.agentMeta && props.resourceSpec) {
    discoverCtx = emptyDiscoverContext({
      agentMeta: props.agentMeta,
      resourceSpec: props.resourceSpec,
    });
  }
  if (props.discoverCtx) {
    discoverCtx = props.discoverCtx;
  }

  return (
    <MemoryRouter
      initialEntries={
        props.initialEntries || [{ pathname: cfg.routes.discover }]
      }
    >
      <ThemeProvider>
        <ContextProvider ctx={ctx}>
          <FeaturesContextProvider value={props.features || []}>
            <InfoGuidePanelProvider>
              <DiscoverProvider mockCtx={discoverCtx}>
                <PingTeleportProvider
                  interval={props.interval || 100000}
                  resourceKind={props.resourceSpec?.kind}
                >
                  <DiscoverBox>{props.children}</DiscoverBox>
                </PingTeleportProvider>
              </DiscoverProvider>
            </InfoGuidePanelProvider>
          </FeaturesContextProvider>
        </ContextProvider>
      </ThemeProvider>
    </MemoryRouter>
  );
};

export function emptyDiscoverContext({
  agentMeta,
  resourceSpec,
}: {
  agentMeta?: AgentMeta;
  resourceSpec?: SelectResourceSpec;
}): DiscoverContextState {
  return {
    agentMeta: agentMeta
      ? agentMeta
      : { resourceName: '', agentMatcherLabels: [] },
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: resourceSpec ? resourceSpec : emptyResourceSpec(null),
  };
}

export function emptyResourceSpec(kind: ResourceKind): SelectResourceSpec {
  return {
    name: '',
    kind,
    icon: null,
    keywords: [],
    event: null,
    id: null,
  };
}

export const resourceSpecAwsEks = KUBERNETES.find(
  k => k.id === DiscoverGuideId.KubernetesAwsEks
);

export const resourceSpecSelfHostedKube = KUBERNETES.find(
  k => k.id === DiscoverGuideId.Kubernetes
);

export const resourceSpecAwsEc2Ssm = SERVERS.find(
  s => s.id === DiscoverGuideId.ServerAwsEc2Ssm
);

export const resourceSpecServerLinuxUbuntu = SERVERS.find(
  s => s.id === DiscoverGuideId.ServerLinuxUbuntu
);

export const resourceSpecConnectMyComputer = SERVERS.find(
  s => s.id === DiscoverGuideId.ConnectMyComputer
);

export const resourceSpecAppAwsCliConsole = APPLICATIONS.find(
  a => a.id === DiscoverGuideId.ApplicationAwsCliConsole
);

export const resourceSpecSamlGcp = SAML_APPLICATIONS.find(
  s => s.id === DiscoverGuideId.ApplicationSamlWorkforceIdentityFederation
);
