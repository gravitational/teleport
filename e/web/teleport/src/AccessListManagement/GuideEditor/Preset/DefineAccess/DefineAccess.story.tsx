import { StoryObj } from '@storybook/react-vite';

import { fetchUnifiedResources, makeHandlers } from '../TestHelper/mocks';
import { Provider } from '../TestHelper/Provider';
import { DefineAccess } from './DefineAccess';
import { NoAccessDefinedDialog } from './NoAccessDefinedDialog';

export default {
  title: 'TeleportE/AccessLists/Guide/DefineAccess',
};

export const WithResourcesInCluster: StoryObj = {
  beforeEach({ msw }) {
    msw.use(...makeHandlers([fetchUnifiedResources('get', null)]));
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
  beforeEach({ msw }) {
    msw.use(...makeHandlers([fetchUnifiedResources('get', [])]));
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
  beforeEach({ msw }) {
    msw.use(...makeHandlers([fetchUnifiedResources('loading')]));
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
  beforeEach({ msw }) {
    msw.use(...makeHandlers([fetchUnifiedResources('any-error')]));
  },
  render() {
    return (
      <Provider>
        <DefineAccess />
      </Provider>
    );
  },
};

export const NoAccessDefinedDialogueCreating: StoryObj = {
  render() {
    return (
      <NoAccessDefinedDialog
        onCancel={() => null}
        onNext={() => null}
        isEditing={false}
      />
    );
  },
};

export const NoAccessDefinedDialogueEditing: StoryObj = {
  render() {
    return (
      <NoAccessDefinedDialog
        onCancel={() => null}
        onNext={() => null}
        isEditing={true}
      />
    );
  },
};
