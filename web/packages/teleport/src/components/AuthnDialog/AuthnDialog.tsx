/**
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

import React from 'react';
import Dialog, {
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogContent,
} from 'design/Dialog';
import { Danger } from 'design/Alert';
import { Text, ButtonPrimary, ButtonSecondary } from 'design';

export default function AuthnDialog({
  onContinue,
  onCancel,
  errorText,
}: Props) {
  return (
    <Dialog dialogCss={() => ({ width: '400px' })} open={true}>
      <DialogHeader style={{ flexDirection: 'column' }}>
        <DialogTitle textAlign="center">
          Multi-factor authentication
        </DialogTitle>
      </DialogHeader>
      <DialogContent mb={6}>
        {errorText && (
          <Danger mt={2} width="100%">
            {errorText}
          </Danger>
        )}
        <Text textAlign="center">
          Re-enter your multi-factor authentication in the browser to continue.
        </Text>
      </DialogContent>
      <DialogFooter textAlign="center">
        <ButtonPrimary onClick={onContinue} autoFocus mr={3} width="130px">
          {errorText ? 'Retry' : 'OK'}
        </ButtonPrimary>
        <ButtonSecondary onClick={onCancel}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

export type Props = {
  onContinue: () => void;
  onCancel: () => void;
  errorText: string;
};
