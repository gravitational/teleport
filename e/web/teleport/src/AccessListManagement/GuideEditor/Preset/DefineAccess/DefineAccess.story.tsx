import { StoryObj } from '@storybook/react-vite';

import { fetchUnifiedResources, makeHandlers } from '../TestHelper/mocks';
import { Provider } from '../TestHelper/Provider';
import { DefineAccess } from './DefineAccess';
import { NoAccessDefinedDialog } from './NoAccessDefinedDialog';

export default {
  title: 'TeleportE/AccessLists/Guide/DefineAccess',
};

export const WithResourcesInCluster: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([fetchUnifiedResources('get', null)]),
    },
  },
  render() {
    return (
      <Provider>
        <DefineAccess />
      </Provider>
    );
  },
};

export const NoResourceInCluster: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([fetchUnifiedResources('get', [])]),
    },
  },
  render() {
    return (
      <Provider>
        <DefineAccess />
      </Provider>
    );
  },
};

export const LoadingResources: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([fetchUnifiedResources('loading')]),
    },
  },
  render() {
    return (
      <Provider>
        <DefineAccess />
      </Provider>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: makeHandlers([fetchUnifiedResources('any-error')]),
    },
  },
  render() {
    return (
      <Provider>
        <DefineAccess />
      </Provider>
    );
  },
};

export const NoAccessDefinedDialogue: StoryObj = {
  render() {
    return <NoAccessDefinedDialog onCancel={() => null} onNext={() => null} />;
  },
};
