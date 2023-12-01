import React, { useEffect } from 'react';
import { MemoryRouter, Route } from 'react-router';

import cfg from 'teleport/config';

import { ContextProvider } from 'teleport';

import TeleportEContext from 'e-teleport/teleportContextE';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { PluginEnroll } from './PluginEnroll';

const defaultIsTeamFlag = cfg.isTeam;

export default {
  title: 'TeleportE/Integrations/Enroll',
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isTeam = defaultIsTeamFlag;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const EnrollSlack = () => renderPluginEnroll('');

export const EnrollMattermost = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('mattermost'));

export const EnrollJamf = () => {
  cfg.isTeam = false;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('jamf'), ctx);
};

export const EnrollJamfDisableInTeam = () => {
  cfg.isTeam = true;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('jamf'), ctx);
};

export const EnrollJira = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('jira'));

export const EnrollPagerduty = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('pagerduty'));

export const EnrollDiscord = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('discord'));

export const EnrollOpsgenie = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('opsgenie'));

export const EnrollOkta = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('okta'));

export const EnrollServiceNow = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('servicenow'));

export const EnrollSuccess = () =>
  renderPluginEnroll(
    `event_id=c6b794e1-afcf-4e16-ac5b-48fe4ba6e54b&success=%7B%22name%22%3A%22slack-default%22%2C%22slack%22%3A%7B%22fallback_channel%22%3A%22%23general%22%7D%7D`
  );

export const EnrollSuccessMalFormedJSON = () =>
  renderPluginEnroll(
    `event_id=c6b794e1-afcf-4e16-ac5b-48fe4ba6e54b&success=%7B%7D`
  );

export const EnrollFailed = () =>
  renderPluginEnroll(
    `event_id=c6b794e1-afcf-4e16-ac5b-48fe4ba6e54b&error=some-error&error_description=some%20error%20 description`
  );

function renderPluginEnroll(
  search: string,
  pathname = cfg.getIntegrationEnrollRoute('slack'),
  ctx?: TeleportEContext
) {
  return (
    <MemoryRouter initialEntries={[{ pathname, search }]}>
      <Route path={cfg.routes.integrationEnroll}>
        <ContextProvider ctx={ctx}>
          <PluginEnroll />
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
}
