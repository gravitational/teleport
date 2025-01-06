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

import { Alert, ButtonSecondary, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import history from 'teleport/services/history';

export function ErrorDialog({ errMsg }: { errMsg: string }) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>An error has occurred</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Alert kind="danger" children={errMsg} />
        <Text mb={3}>Try again by refreshing the page.</Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary
          onClick={() =>
            history.goToLogin({
              rememberLocation: true,
            })
          }
        >
          Go to Login
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
