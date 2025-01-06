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

import { ButtonPrimary, ButtonSecondary, Text } from 'design';
import { Danger } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

export default function HeadlessRequestDialog({
  ipAddress,
  onAccept,
  onReject,
  errorText,
}: Props) {
  return (
    <Dialog dialogCss={() => ({ width: '400px' })} open={true}>
      <DialogHeader style={{ flexDirection: 'column' }}>
        <DialogTitle textAlign="center">
          Host {ipAddress} wants to execute a command
        </DialogTitle>
      </DialogHeader>
      <DialogContent mb={6}>
        {errorText && (
          <Danger mt={2} width="100%">
            {errorText}
          </Danger>
        )}
        <Text textAlign="center">
          {errorText ? (
            <>
              The requested session doesn't exist or is invalid. Please generate
              a new request.
              <br />
              <br />
              You can close this window.
            </>
          ) : (
            <>
              Someone has initiated a command from {ipAddress}. If it was not
              you, click Reject and contact your administrator.
              <br />
              <br />
              If it was you, please use your hardware key to approve.
            </>
          )}
        </Text>
      </DialogContent>
      <DialogFooter textAlign="center">
        {!errorText && (
          <>
            <ButtonPrimary onClick={onAccept} autoFocus mr={3} width="130px">
              Approve
            </ButtonPrimary>
            <ButtonSecondary onClick={onReject}>Reject</ButtonSecondary>
          </>
        )}
      </DialogFooter>
    </Dialog>
  );
}

export type Props = {
  ipAddress: string;
  onAccept: () => void;
  onReject: () => void;
  errorText: string;
};
