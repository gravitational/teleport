import React from 'react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';

import {
  DiscoverContextState,
  DiscoverProvider,
  SamlGcpWorkforceMeta,
} from 'teleport/Discover/useDiscover';

import cfg from 'teleport/config';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { SamlServiceProviderPreset } from 'teleport/Discover/SelectResource/types';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { ConfigurePool } from './ConfigureWorkforcePool';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';
import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

describe('Configure GCP workforce pool', () => {
  const Provider = props => {
    const ctx = createTeleportContextE({ customAcl: props.customAcl });
    const discoverCtx: DiscoverContextState = {
      nextStep: () => null,
      prevStep: () => null,
      agentMeta: {
        isAutoConfig: true,
        orgId: '',
        poolName: '',
        poolProviderName: '',
      },
      updateAgentMeta: SamlGcpWorkforceMeta => SamlGcpWorkforceMeta,
      resourceSpec: {
        samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
      } as ResourceSpec,
      currentStep: 0,
      onSelectResource: () => null,
      exitFlow: () => null,
      viewConfig: null,
      indexedViews: [],
      setResourceSpec: () => null,
      emitErrorEvent: () => null,
      emitEvent: () => null,
      eventState: null,
    };

    return (
      <MemoryRouter
        initialEntries={[
          { pathname: cfg.routes.discover, state: { entity: 'app' } },
        ]}
      >
        <ContextProvider ctx={ctx}>
          <DiscoverProvider mockCtx={discoverCtx}>
            {props.children}
          </DiscoverProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  test('Toggle on and off enables and disales auto config flow', async () => {
    const mockFetchMetadata = jest.fn().mockResolvedValue({
      entityID: '',
      ssoURL: '',
      x509PEM: '',
    } as SAMLIdPMetadataResponse);

    render(
      <Provider>
        <ConfigurePool
          agentMeta={{} as SamlGcpWorkforceMeta}
          updateAgentMeta={jest.fn()}
          resourceSpec={
            {
              samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
            } as ResourceSpec
          }
          prevStep={() => null}
          fetchMetadata={mockFetchMetadata}
          nextStep={() => null}
        />
      </Provider>
    );

    const guidedFlowEl = screen.getByText(
      'Guided configuration flow is enabled.'
    );
    expect(guidedFlowEl).toBeInTheDocument();

    expect(
      screen.getByText(
        'Generate script to configure Workforce Identity Federation pool and pool provider'
      )
    ).toBeInTheDocument();

    fireEvent.click(guidedFlowEl);

    await waitFor(() => {
      expect(
        screen.getByText('Guided configuration flow is disabled.')
      ).toBeInTheDocument();
    });

    expect(
      screen.getByText('Guided configuration flow is disabled.')
    ).toBeInTheDocument();

    expect(screen.getByText('Teleport IdP Metadata')).toBeInTheDocument();
  });
});
