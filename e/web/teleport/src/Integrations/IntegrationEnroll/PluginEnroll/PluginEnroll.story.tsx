import { useEffect, useState } from 'react';

import cfg from 'teleport/config';

import { renderPluginEnroll } from './StorybookHelper';

export default {
  title: 'TeleportE/Integrations/Enroll',
};

export const EnrollSlack = () => renderPluginEnroll('');

export const EnrollMattermost = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('mattermost'));

export const EnrollJamf = () => {
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('jamf'));
};

export const EnrollIntune = () => {
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('intune'));
};

export const EnrollJira = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('jira'));

export const EnrollPagerduty = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('pagerduty'));

export const EnrollDiscord = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('discord'));

export const EnrollOpsgenie = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('opsgenie'));

export const EnrollServiceNow = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('servicenow'));

export const EnrollDatadog = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('datadog'));

export const EnrollMsteams = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('msteams'));

export const EnrollEmail = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('email'));

export const EnrollEmailCloud = () => {
  const [, setState] = useState({});

  useEffect(() => {
    const defaultIsCloud = cfg.isCloud;
    cfg.isCloud = true;
    setState({}); // Rerender component with updated cfg.
    return () => {
      cfg.isCloud = defaultIsCloud;
    };
  }, []);

  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('email'));
};

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
