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

import { FC } from 'react';

import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { ButtonIcon, ButtonSecondary, Text, Flex, H2 } from 'design';
import * as icons from 'design/Icon';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { routing } from 'teleterm/ui/uri';
import { PromptHardwareKeyTouchRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

export const HardwareKeyTouch: FC<{
  req: PromptHardwareKeyTouchRequest;
  onCancel: () => void;
}> = props => {
  const { rootClusterUri } = props.req;
  const { clustersService } = useAppContext();
  // TODO(ravicious): Use a profile name here from the URI and remove the dependency on
  // clustersService. https://github.com/gravitational/teleport/issues/33733
  // const rootClusterUri = routing.ensureRootClusterUri(clusterUri);
  const rootClusterName =
    clustersService.findRootClusterByResource(rootClusterUri)?.name ||
    routing.parseClusterName(rootClusterUri);

  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
        }}
      >
        <DialogHeader
          justifyContent="space-between"
          mb={0}
          alignItems="baseline"
        >
          <H2 mb={4}>
            Unlock hardware key to access <strong>{rootClusterName}</strong>
          </H2>

          <ButtonIcon
            type="button"
            onClick={props.onCancel}
            color="text.slightlyMuted"
          >
            <icons.Cross size="medium" />
          </ButtonIcon>
        </DialogHeader>

        <DialogContent mb={4}>
          <Flex flexDirection="column" gap={4} alignItems="flex-start">
            <Text color="text.slightlyMuted">Touch your hardware key</Text>
          </Flex>
        </DialogContent>

        <DialogFooter>
          <Flex gap={3}>
            <ButtonSecondary type="button" onClick={props.onCancel}>
              Cancel
            </ButtonSecondary>
          </Flex>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
};
