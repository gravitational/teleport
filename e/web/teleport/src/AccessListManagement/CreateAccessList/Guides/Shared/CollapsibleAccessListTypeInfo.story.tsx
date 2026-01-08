import { Meta } from '@storybook/react-vite';

import { UserType, UserTypeOption } from '../../types';
import { CollapsibleAccessListTypeInfo } from './CollapsibleAccessListTypeInfo';

type StoryProps = {
  userCategory: 'owner' | 'member';
  userType: UserType;
};

const meta: Meta<StoryProps> = {
  title: 'TeleportE/AccessLists/Guide',
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
    userType: 'okta-access-lists',
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
    <CollapsibleAccessListTypeInfo
      userCategory={props.userCategory}
      userTypeOption={userTypeOption}
    />
  );
}
