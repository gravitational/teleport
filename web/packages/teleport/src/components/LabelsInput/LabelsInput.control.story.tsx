/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
import { useState } from 'react';

import Validation from 'shared/components/Validation';

import { Label, LabelsInput } from './LabelsInput';

type StoryProps = {
  readOnly?: boolean;
  disableBtns?: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/LabelsInput/AtLeastOneRowVisible',
  component: Controls,
  argTypes: {
    readOnly: {
      control: { type: 'boolean' },
    },
    disableBtns: {
      control: { type: 'boolean' },
    },
  },
};
export default meta;

export function Controls(props: StoryProps) {
  const [labels, setLabels] = useState<Label[]>([]);
  return (
    <Validation>
      <LabelsInput
        legend="Labels"
        labels={labels}
        setLabels={setLabels}
        atLeastOneRow
        readOnly={props.readOnly}
        disableBtns={props.disableBtns}
      />
    </Validation>
  );
}
