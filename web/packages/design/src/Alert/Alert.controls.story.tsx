/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Meta } from '@storybook/react';

import { Alert, AlertKind, AlertProps } from './Alert';

type StoryProps = {
  kind: AlertKind;
  children: string;
  details: string;
  dismissible: boolean;
  primaryAction: string;
  secondaryAction: string;
  alignItems: AlertProps['alignItems'];
};

const meta: Meta<StoryProps> = {
  title: 'Design/Alerts/Controls',
  component: Controls,
  argTypes: {
    kind: {
      control: { type: 'select' },
      options: [
        'neutral',
        'danger',
        'info',
        'warning',
        'success',
        'outline-danger',
        'outline-info',
        'outline-warn',
      ],
    },
    alignItems: {
      control: { type: 'radio' },
      options: ['center', 'flex-start'],
    },
  },
  args: {
    kind: 'neutral',
    children: 'Lorem ipsum dolor sit amet',
    details: 'Maecenas ut scelerisque nunc, blandit porta est.',
    dismissible: true,
    primaryAction: 'Primary Action',
    secondaryAction: 'Secondary Action',
    alignItems: 'center',
  },
};
export default meta;

export function Controls(props: StoryProps) {
  return (
    <Alert
      kind={props.kind}
      details={props.details}
      dismissible={props.dismissible}
      primaryAction={
        props.primaryAction ? { content: props.primaryAction } : undefined
      }
      secondaryAction={
        props.secondaryAction ? { content: props.secondaryAction } : undefined
      }
      alignItems={props.alignItems}
    >
      {props.children}
    </Alert>
  );
}
