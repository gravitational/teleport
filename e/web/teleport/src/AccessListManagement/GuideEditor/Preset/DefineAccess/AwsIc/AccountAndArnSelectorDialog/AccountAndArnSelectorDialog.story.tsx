import { StoryObj } from '@storybook/react-vite';

import { Info } from 'design/Alert';

import cfg from 'e-teleport/config';

import {
  appsWithAllMatchingPermissionSet,
  appsWithoutPermissionSets,
  fetchUnifiedResources,
  makeHandlers,
} from '../../../TestHelper/mocks';
import { Provider } from '../../../TestHelper/Provider';
import { AccountAndArnSelectorDialog } from './AccountAndArnSelectorDialog';

export default {
  title:
    'TeleportE/AccessLists/Guide/DefineAccess/AwsIcSection/SelectionDialog',
};

export const NoPermissionSets: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      ...makeHandlers([fetchUnifiedResources('get', appsWithoutPermissionSets)])
    );
  },
  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

    return (
      <Provider>
        <Info>Devs: Select accounts to see different permission set state</Info>
        <AccountAndArnSelectorDialog onClose={() => null} />
      </Provider>
    );
  },
};

export const WithPermissionSets: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      ...makeHandlers([
        fetchUnifiedResources('get', appsWithAllMatchingPermissionSet),
      ])
    );
  },
  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

    return (
      <Provider>
        <Info>Devs: Select accounts to see different permission set state</Info>
        <AccountAndArnSelectorDialog onClose={() => null} />
      </Provider>
    );
  },
};

export const FetchFailed: StoryObj = {
  beforeEach({ msw }) {
    msw.use(...makeHandlers([fetchUnifiedResources('any-error')]));
  },
  render() {
    return (
      <Provider>
        <AccountAndArnSelectorDialog onClose={() => null} />
      </Provider>
    );
  },
};

export const Processing: StoryObj = {
  beforeEach({ msw }) {
    msw.use(...makeHandlers([fetchUnifiedResources('loading')]));
  },
  render() {
    return (
      <Provider>
        <AccountAndArnSelectorDialog onClose={() => null} />
      </Provider>
    );
  },
};
