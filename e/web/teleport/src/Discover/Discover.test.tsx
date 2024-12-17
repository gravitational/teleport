import { MemoryRouter } from 'react-router';
import { fireEvent, render, screen } from 'design/utils/testing';
import { ResourceKind } from 'teleport/Discover/Shared';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getAcl } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import { getEnterpriseFeatures } from 'e-teleport/features';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { idpMetadata } from 'e-teleport/SamlApplication/fixtures';

import { Discover } from './Discover';

const renderDiscover = () => {
  const defaultPref = makeDefaultUserPreferences();
  defaultPref.onboard.preferredResources = [];
  mockUserContextProviderWith(
    makeTestUserContext({ preferences: defaultPref })
  );
  const ctx = createTeleportContextE({ customAcl: getAcl() });
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  ctx.storeAccessRequests.getSessionExpiry = () => Promise.resolve(null);

  // TODO(sshah): update Discover flow to use "cfg.edition" instead of "cfg.isEnterprise"
  cfg.isEnterprise = true;

  return render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: '' } },
      ]}
    >
      <TeleportContextProvider ctx={ctx}>
        <FeaturesContextProvider value={getEnterpriseFeatures()}>
          <Discover />
        </FeaturesContextProvider>
      </TeleportContextProvider>
    </MemoryRouter>
  );
};

test('displays all resources by default', () => {
  renderDiscover();

  expect(screen.getAllByTestId(ResourceKind.SamlApplication)).toHaveLength(3);

  const samlGenericEl = screen.getByText('SAML Application (Generic)');
  expect(samlGenericEl).toBeInTheDocument();
  fireEvent.click(samlGenericEl);
  expect(
    screen.getByText(
      `Configure Service Provider with Teleport's Identity Provider Metadata`
    )
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: /Back/i }));

  const samlGcpWorkforceEl = screen.getByText('Workforce Identity Federation');
  expect(samlGcpWorkforceEl).toBeInTheDocument();
  fireEvent.click(samlGcpWorkforceEl);
  expect(
    screen.getByText(`Configure Workforce Pool Provider in GCP`)
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: /Back/i }));

  const samlGrafanaEl = screen.getByText('Grafana');
  expect(samlGrafanaEl).toBeInTheDocument();
  fireEvent.click(samlGrafanaEl);
  expect(
    screen.getByText(`Configure Grafana with Teleport's IdP Metadata`)
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: /Back/i }));
});
