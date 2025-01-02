import { useEffect } from 'react';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import cfg from 'teleport/config';

import { renderPluginEnroll } from './StorybookHelper';

const defaultMdmFlag = cfg.entitlements.MobileDeviceManagement;
const defaultIsEnterprise = cfg.isEnterprise;

export default {
  title: 'TeleportE/Integrations/Enroll',
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.entitlements.MobileDeviceManagement = defaultMdmFlag;
          cfg.isEnterprise = defaultIsEnterprise;
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
  cfg.entitlements.MobileDeviceManagement = { enabled: true, limit: 0 };
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('jamf'), ctx);
};

export const EnrollJamfDisabled = () => {
  cfg.entitlements.MobileDeviceManagement = { enabled: false, limit: 0 };
  cfg.isEnterprise = true;
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

export const EnrollServiceNow = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('servicenow'));

export const EnrollDatadog = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('datadog'));

export const EnrollMsteams = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('msteams'));

export const EnrollEmail = () =>
  renderPluginEnroll('', cfg.getIntegrationEnrollRoute('email'));

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
