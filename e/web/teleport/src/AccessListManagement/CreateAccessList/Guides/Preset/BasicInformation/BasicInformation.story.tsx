import { StoryObj } from '@storybook/react-vite';
import { useEffect } from 'react';

import cfg from 'e-teleport/config';

import { ComponentWithPreset, Provider, sharedHandlers } from '../storyHelper';
import { BasicInformation as Component } from './BasicInformation';

const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultAccessListEntitlement = cfg.oss.entitlements.AccessLists;

export default {
  title: 'TeleportE/AccessLists/Guide',
  decorators: [
    Story => {
      cfg.oss.isEnterprise = true;
      cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

      // Reset request handlers added in individual stories.
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isEnterprise = defaultIsEnterprise;
          cfg.oss.entitlements.AccessLists = defaultAccessListEntitlement;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const BasicInformation: StoryObj = {
  beforeEach({ msw }) {
    msw.use(...sharedHandlers);
  },
  render() {
    return (
      <Provider>
        <ComponentWithPreset preset="long-term">
          <Component />
        </ComponentWithPreset>
      </Provider>
    );
  },
};
