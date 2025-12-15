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
import { H1 } from 'design/Text';

import { FieldRadio as Component } from './FieldRadio';

type StoryProps = {
  readOnly?: boolean;
  disabled?: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Shared',
  component: FieldRadio,
  args: {
    readOnly: false,
    disabled: false,
  },
};
export default meta;

export function FieldRadio(props: StoryProps) {
  return (
    <Box width={600}>
      <H1 mb={2}>Group 1</H1>
      <Component
        name="grp1"
        label="Unchecked radio button"
        defaultChecked={false}
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        name="grp1"
        label="Checked radio button"
        defaultChecked={true}
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        name="grp1"
        size="small"
        label="Small radio button"
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <H1 mb={2}>Group 2</H1>
      <Component
        name="grp2"
        label="Radio button with helper text"
        helperText="I'm a helpful helper text"
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        name="grp2"
        size="small"
        label="Small radio button with helper text"
        helperText="Another helpful helper text"
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
      <Component
        name="grp2"
        label="You must choose. But choose wisely, for while the true Grail will
      bring you life, the false Grail will take it from you."
        helperText="I was chosen because I was the bravest and the most worthy.
      The honor was mine until another came to challenge me to single combat. I
      pass it to you who vanquished me."
        disabled={props.disabled}
        readOnly={props.readOnly}
      />
    </Box>
  );
}
