import { screen, userEvent } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { getEnterpriseFeatures } from 'e-teleport/features';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { idpMetadata } from 'e-teleport/SamlApplication/fixtures';
import cfg from 'teleport/config';
import { ResourceKind } from 'teleport/Discover/Shared';
import { getGuideTileId } from 'teleport/Discover/testUtils';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getAcl } from 'teleport/mocks/contexts';
import { userEventService } from 'teleport/services/userEvent';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { renderWithMemoryRouter } from 'teleport/test/helpers/router';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

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

  return renderWithMemoryRouter(
    <TeleportContextProvider ctx={ctx}>
      <FeaturesContextProvider value={getEnterpriseFeatures()}>
        <InfoGuidePanelProvider>
          <Discover />
        </InfoGuidePanelProvider>
      </FeaturesContextProvider>
    </TeleportContextProvider>,
    {
      initialEntries: [
        { pathname: cfg.routes.discover, state: { entity: '' } },
      ],
    }
  );
};

test('displays all resources by default', async () => {
  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(null as never); // return value does not matter but required by ts

  const user = userEvent.setup();
  renderDiscover();

  expect(
    screen.getAllByTestId(
      getGuideTileId({ kind: ResourceKind.SamlApplication })
    )
  ).toHaveLength(4);

  const samlGenericEl = screen.getByText('SAML Application (Generic)');
  expect(samlGenericEl).toBeInTheDocument();
  await user.click(samlGenericEl);
  expect(
    screen.getByText(
      `Configure Service Provider with Teleport's Identity Provider Metadata`
    )
  ).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: /Back/i }));

  const samlGcpWorkforceEl = screen.getByText('Workforce Identity Federation');
  expect(samlGcpWorkforceEl).toBeInTheDocument();
  await user.click(samlGcpWorkforceEl);
  expect(
    screen.getByText(`Configure Workforce Pool Provider in GCP`)
  ).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: /Back/i }));

  const samlGrafanaEl = screen.getByText(/grafana/i);
  expect(samlGrafanaEl).toBeInTheDocument();
  await user.click(samlGrafanaEl);
  expect(
    screen.getByText(`Configure Grafana with Teleport's IdP Metadata`)
  ).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: /Back/i }));

  const samlMicrosoftEntraIdEl = screen.getByText(
    'Microsoft Entra External ID'
  );
  expect(samlMicrosoftEntraIdEl).toBeInTheDocument();
  await user.click(samlMicrosoftEntraIdEl);
  await screen.findAllByText(
    'Configure Teleport as an identity provider for Microsoft Entra External ID'
  );
  await user.click(screen.getByRole('button', { name: /Back/i }));
});
