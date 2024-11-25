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

import { ButtonPrimary } from '../Button';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './index';

export default {
  title: 'Design/Dialog/Confirmation',
};

export const Confirmation = () => (
  <DialogConfirmation open={true}>
    <DialogHeader>
      <DialogTitle>Confirmation Dialog Header</DialogTitle>
    </DialogHeader>
    <DialogContent>Simplified dialog for use with confirmations</DialogContent>
    <DialogFooter>
      <ButtonPrimary>Save and Close</ButtonPrimary>
    </DialogFooter>
  </DialogConfirmation>
);
