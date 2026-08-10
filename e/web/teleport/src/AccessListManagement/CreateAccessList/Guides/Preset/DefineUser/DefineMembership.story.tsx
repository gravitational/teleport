import { StoryObj } from '@storybook/react-vite';
import { useEffect } from 'react';

import cfg from 'e-teleport/config';

import {
  ComponentWithPreset,
  Provider,
  ProviderWithOktaOrigin,
  sharedHandlers,
} from '../storyHelper';
import { DefineMembership } from './DefineMembership';

const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultAccessListEntitlement = cfg.oss.entitlements.AccessLists;

export default {
  title: 'TeleportE/AccessLists/Guide/DefineMembership',
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

export const LongTerm: StoryObj = {
  beforeEach({ msw }) {
    msw.use(...sharedHandlers);
  },
  render() {
    return (
      <Provider>
        <ComponentWithPreset preset="long-term">
          <DefineMembership />
        </ComponentWithPreset>
      </Provider>
    );
  },
};

export const ShortTerm: StoryObj = {
  beforeEach({ msw }) {
    msw.use(...sharedHandlers);
  },
  render() {
    return (
      <Provider>
        <ComponentWithPreset preset="short-term">
          <DefineMembership />
        </ComponentWithPreset>
      </Provider>
    );
  },
};

export const OktaOriginNoAccessLists: StoryObj = {
  beforeEach({ msw }) {
    msw.use(...sharedHandlers);
  },
  render() {
    return (
      <ProviderWithOktaOrigin>
        <ComponentWithPreset preset="long-term">
          <DefineMembership />
        </ComponentWithPreset>
      </ProviderWithOktaOrigin>
    );
  },
};
