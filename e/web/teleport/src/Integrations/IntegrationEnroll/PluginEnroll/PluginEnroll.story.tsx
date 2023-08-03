import React from 'react';
import { MemoryRouter, Route } from 'react-router';

import cfg from 'teleport/config';

import { PluginEnroll } from './PluginEnroll';

export default {
  title: 'TeleportE/Integrations/Enroll',
};

export const EnrollSlack = () => renderPluginEnroll('');

export const EnrollMattermost = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('mattermost'));

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
  pathname = cfg.getIntegrationEnrollRoute('slack')
) {
  return (
    <MemoryRouter initialEntries={[{ pathname, search }]}>
      <Route path={cfg.routes.integrationEnroll}>
        <PluginEnroll />
      </Route>
    </MemoryRouter>
  );
}
