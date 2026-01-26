import { StoryObj } from '@storybook/react-vite';

import { NoAccessDefinedDialog } from './NoAccessDefinedDialog';

export default {
  title: 'TeleportE/AccessLists/Guide/DefineAccess',
};

export const NoAccessDefinedDialogue: StoryObj = {
  render() {
    return <NoAccessDefinedDialog onCancel={() => null} onNext={() => null} />;
  },
};
