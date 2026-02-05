import { Meta } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';

import { UserType, UserTypeOption } from '../../types';
import { CollapsibleAccessListTypeInfo } from './CollapsibleAccessListTypeInfo';
import { UserCategory } from './types';

type StoryProps = {
  userCategory: UserCategory;
  userType: UserType;
  hasOktaPlugin: boolean;
  hasOktaAppGroupSyncEnabled: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'TeleportE/AccessLists/Guide/UserTypeInfo',
  argTypes: {
    userCategory: {
      control: { type: 'select' },
      options: ['owner', 'member'],
    },
    userType: {
      control: { type: 'select' },
      options: ['access-lists', 'okta-access-lists', 'users'],
    },
  },
  // default
  args: {
    userCategory: 'member',
    userType: 'access-lists',
    hasOktaPlugin: true,
    hasOktaAppGroupSyncEnabled: true,
  },
};
export default meta;

export function AccessListTypeInfo(props: StoryProps) {
  let userTypeOption: UserTypeOption;
  switch (props.userType) {
    case 'access-lists':
      userTypeOption = {
        value: 'access-lists',
        label: 'Access Lists',
      };
      break;
    case 'okta-access-lists':
      userTypeOption = {
        value: 'okta-access-lists',
        label: 'Okta Access Lists',
      };
      break;
    case 'users':
      userTypeOption = {
        value: 'users',
        label: 'Users',
      };
      break;
  }

  return (
    <MemoryRouter>
      <CollapsibleAccessListTypeInfo
        userCategory={props.userCategory}
        userTypeOption={userTypeOption}
        okta={{
          hasPlugin: props.hasOktaPlugin,
          hasAppGroupSyncEnabled: props.hasOktaAppGroupSyncEnabled,
          hasConfiguredOauthCredentials: true,
        }}
      />
    </MemoryRouter>
  );
}
