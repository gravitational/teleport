import { render, screen } from 'design/utils/testing';
import { MemoryRouter, Route } from 'react-router';
import { IntegrationKind } from 'teleport/services/integrations';

import cfg from 'teleport/config';

import { IntegrationStatus } from 'e-teleport/Integrations/IntegrationStatus/IntegrationStatus';

test('okta does not show unsupported message', () => {
  render(
    <MemoryRouter initialEntries={[`/web/integrations/status/okta/some-name`]}>
      <Route path={cfg.routes.integrationStatus}>
        <IntegrationStatus />
      </Route>
    </MemoryRouter>
  );

  expect(
    screen.queryByText(`Status for integration type okta is not supported`)
  ).not.toBeInTheDocument();
  expect(screen.getByText('Okta Integration')).toBeInTheDocument();
});

test('unsupported integration kinds', () => {
  for (const key in IntegrationKind) {
    render(
      <MemoryRouter
        initialEntries={[`/web/integrations/status/${key}/some-name`]}
      >
        <Route path={cfg.routes.integrationStatus}>
          <IntegrationStatus />
        </Route>
      </MemoryRouter>
    );

    expect(
      screen.getByText(`Status for integration type ${key} is not supported`)
    ).toBeInTheDocument();
  }
});

test.each`
  type
  ${'slack'}
  ${'openai'}
  ${'pagerduty'}
  ${'email'}
  ${'jira'}
  ${'discord'}
  ${'mattermost'}
  ${'msteams'}
  ${'opsgenie'}
  ${'servicenow'}
  ${'jamf'}
  ${'entra-id'}
  ${'datadog'}
  ${'aws-identity-center'}
`('unsupported plugin kind $type', async ({ type }) => {
  render(
    <MemoryRouter
      initialEntries={[`/web/integrations/status/${type}/some-name`]}
    >
      <Route path={cfg.routes.integrationStatus}>
        <IntegrationStatus />
      </Route>
    </MemoryRouter>
  );

  expect(
    screen.getByText(`Status for integration type ${type} is not supported`)
  ).toBeInTheDocument();
});
