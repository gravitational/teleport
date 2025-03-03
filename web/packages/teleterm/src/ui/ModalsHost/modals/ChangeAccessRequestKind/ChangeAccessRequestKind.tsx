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

import { ButtonIcon, ButtonWarning, H2 } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';
import { P } from 'design/Text/Text';

export function ChangeAccessRequestKind({
  onCancel,
  onConfirm,
  hidden,
}: {
  onCancel(): void;
  onConfirm(): void;
  hidden?: boolean;
}) {
  return (
    <DialogConfirmation
      open={!hidden}
      keepInDOMAfterClose
      onClose={onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={4}>
        <H2>Replace selected resources?</H2>
        <ButtonIcon onClick={onCancel} color="text.slightlyMuted">
          <Cross size="small" />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={4} color="text.slightlyMuted">
        <P>
          Resource Access Request cannot be combined with Role Access Request.
          The current items will be cleared.
        </P>
        <P>Do you want to continue?</P>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning size="large" block={true} onClick={onConfirm}>
          Continue
        </ButtonWarning>
      </DialogFooter>
    </DialogConfirmation>
  );
}
