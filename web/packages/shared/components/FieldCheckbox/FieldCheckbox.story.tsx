/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Meta } from '@storybook/react-vite';

import Box from 'design/Box';

import { FieldCheckbox as Component } from '.';

type StoryProps = {
  readOnly?: boolean;
  disabled?: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Shared',
  component: FieldCheckbox,
  args: {
    readOnly: false,
    disabled: false,
  },
};
export default meta;

export function FieldCheckbox(props: StoryProps) {
  return (
    <Box width={600}>
      <Component
        label="Unchecked checkbox"
        defaultChecked={false}
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        label="Checked checkbox"
        defaultChecked={true}
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        size="small"
        label="Small checkbox"
        defaultChecked={true}
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        label="Checkbox with helper text"
        helperText="I'm a helpful helper text"
        defaultChecked={true}
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        size="small"
        label="Small checkbox with helper text"
        helperText="Another helpful helper text"
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        label="To check, or not to check: that is the question:
      Whether 'tis nobler in the mind to suffer
      The slings and arrows of outrageous fortune,
      Or to take arms against a sea of troubles,
      And by opposing end them?"
        helperText="To uncheck: to sleep;
      No more; and by a sleep to say we end
      The heart-ache and the thousand natural shocks
      That flesh is heir to, 'tis a consummation
      Devoutly to be wish'd."
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
    </Box>
  );
}
