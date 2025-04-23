/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { ButtonPrimary, Input, LabelInput } from './..';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './index';

export default {
  title: 'Design/Dialog/Basic',
};

export const Basic = () => (
  <Dialog open={true}>
    <DialogHeader>
      <DialogTitle>Header title</DialogTitle>
    </DialogHeader>
    <DialogContent width="400px">
      <p>Some text and other elements inside content</p>
      <LabelInput>Input</LabelInput>
      <Input />
    </DialogContent>
    <DialogFooter>
      <ButtonPrimary onClick={() => null}>Save And Close</ButtonPrimary>
    </DialogFooter>
  </Dialog>
);
