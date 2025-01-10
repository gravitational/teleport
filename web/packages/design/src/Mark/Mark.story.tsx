/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { IconTooltip } from 'design/Tooltip';

import { Mark as M, MarkInverse } from './Mark';

export default {
  title: 'Design/Mark',
};

export const SampleText = () => {
  return (
    <Box width="500px">
      Some texts to demonstrate word <M>markings</M>. Lorem ipsum dolor sit amet{' '}
      <M>consectetur</M> adipisicing <M>elit</M>. Quidem <M>corrupti</M>,{' '}
      reprehenderit{' '}
      <M>
        <i>maxime</i>
      </M>{' '}
      rerum quam{' '}
      <M>
        <b>necessitatibus</b>
      </M>{' '}
      obcaecati asperiores neque.
    </Box>
  );
};

export const MarkInsideTooltip = () => {
  return (
    <IconTooltip>
      Example of <MarkInverse>MarkInverse</MarkInverse> component. Note the{' '}
      <MarkInverse>inversed</MarkInverse> background and text color.
    </IconTooltip>
  );
};
