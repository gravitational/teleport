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
import { H1 } from 'design/Text';

import { FieldRadio } from './FieldRadio';

export default {
  title: 'Shared',
};

export const FieldRadioStory = () => (
  <Box width={600}>
    <H1 mb={2}>Group 1</H1>
    <FieldRadio
      name="grp1"
      label="Unchecked radio button"
      defaultChecked={false}
    />
    <FieldRadio
      name="grp1"
      label="Checked radio button"
      defaultChecked={true}
    />
    <FieldRadio name="grp1" label="Disabled radio button" disabled />
    <FieldRadio name="grp1" size="small" label="Small radio button" />
    <H1 mb={2}>Group 2</H1>
    <FieldRadio
      name="grp2"
      label="Radio button with helper text"
      helperText="I'm a helpful helper text"
    />
    <FieldRadio
      name="grp2"
      size="small"
      label="Small radio button with helper text"
      helperText="Another helpful helper text"
    />
    <FieldRadio
      name="grp2"
      disabled
      label="Disabled radio button with helper text"
      helperText="There's nothing you can do here"
      defaultChecked={true}
    />
    <FieldRadio
      name="grp2"
      label="You must choose. But choose wisely, for while the true Grail will
      bring you life, the false Grail will take it from you."
      helperText="I was chosen because I was the bravest and the most worthy.
      The honor was mine until another came to challenge me to single combat. I
      pass it to you who vanquished me."
    />
  </Box>
);

FieldRadioStory.storyName = 'FieldRadio';
