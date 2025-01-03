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

import Box from 'design/Box';

import { FieldCheckbox } from '.';

export default {
  title: 'Shared',
};

export const FieldCheckboxStory = () => (
  <Box width={600}>
    <FieldCheckbox label="Unchecked checkbox" defaultChecked={false} />
    <FieldCheckbox label="Checked checkbox" defaultChecked={true} />
    <FieldCheckbox label="Disabled checkbox" disabled />
    <FieldCheckbox size="small" label="Small checkbox" defaultChecked={true} />
    <FieldCheckbox
      label="Checkbox with helper text"
      helperText="I'm a helpful helper text"
      defaultChecked={true}
    />
    <FieldCheckbox
      size="small"
      label="Small checkbox with helper text"
      helperText="Another helpful helper text"
    />
    <FieldCheckbox
      disabled
      label="Disabled checkbox with helper text"
      helperText="There's nothing you can do here"
      defaultChecked={true}
    />
    <FieldCheckbox
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
    />
  </Box>
);

FieldCheckboxStory.storyName = 'FieldCheckbox';
