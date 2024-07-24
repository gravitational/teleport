import React, { useEffect } from 'react';
import cfg from 'teleport/config';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { renderPluginEnroll } from './StorybookHelper';

const defaultMdmFlag = cfg.mobileDeviceManagement;
const defaultIsEnterprise = cfg.isEnterprise;
const defaultIgs = cfg.isIgsEnabled;

export default {
  title: 'TeleportE/Integrations/Enroll',
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.mobileDeviceManagement = defaultMdmFlag;
          cfg.isEnterprise = defaultIsEnterprise;
          cfg.isIgsEnabled = defaultIgs;
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
  cfg.mobileDeviceManagement = true;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('jamf'), ctx);
};

export const EnrollJamfDisabled = () => {
  cfg.mobileDeviceManagement = false;
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
