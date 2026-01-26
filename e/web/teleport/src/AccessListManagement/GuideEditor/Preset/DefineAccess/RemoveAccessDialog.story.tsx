import { Meta } from '@storybook/react-vite';

import {
  definableResourceAccessFields,
  DefinableResourceAccessFields,
} from '../role/listaccess';
import { RemoveAccessDialog as Component } from './RemoveAccessDialog';

type StoryProps = {
  accessField: DefinableResourceAccessFields;
};

const meta: Meta<StoryProps> = {
  title: 'TeleportE/AccessLists/Guide/DefineAccess',
  argTypes: {
    accessField: {
      control: { type: 'select' },
      options: definableResourceAccessFields,
    },
  },
  // default
  args: {
    accessField: 'app_labels',
  },
};
export default meta;

export function RemoveAccessDialog(props: StoryProps) {
  return <Component onClose={() => null} field={props.accessField} />;
}
