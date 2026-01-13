import { StoryObj } from '@storybook/react-vite';

import {
  appsWithAllMatchingPermissionSet,
  appsWithSomePermissionSets,
  ComponentWithAccountsSelected,
  fetchUnifiedResources,
  makeHandlers,
  Provider,
} from '../../testHelper';
import { AwsIcSection } from './AwsIcSection';

export default {
  title: 'TeleportE/AccessLists/Guide/DefineAccess/AwsIcSection',
};

export const NoResourcesCta: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([fetchUnifiedResources('get')]),
    },
  },
  render() {
    return (
      <Provider>
        <AwsIcSection />
      </Provider>
    );
  },
};

export const NoSelection: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([
        fetchUnifiedResources('get', appsWithSomePermissionSets),
      ]),
    },
  },
  render() {
    return (
      <Provider>
        <AwsIcSection />
      </Provider>
    );
  },
};

export const WithSelection: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([
        fetchUnifiedResources('get', appsWithSomePermissionSets),
      ]),
    },
  },
  render() {
    return (
      <Provider>
        <ComponentWithAccountsSelected>
          <AwsIcSection />
        </ComponentWithAccountsSelected>
      </Provider>
    );
  },
};

export const WithWildcardSelection: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([
        fetchUnifiedResources('get', appsWithAllMatchingPermissionSet),
      ]),
    },
  },
  render() {
    return (
      <Provider>
        <ComponentWithAccountsSelected wantWildcard={true}>
          <AwsIcSection />
        </ComponentWithAccountsSelected>
      </Provider>
    );
  },
};

export const Loading: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([fetchUnifiedResources('loading')]),
    },
  },
  render() {
    return (
      <Provider>
        <AwsIcSection />
      </Provider>
    );
  },
};
