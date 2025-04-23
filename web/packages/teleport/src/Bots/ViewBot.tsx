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

import { ButtonSecondary, Flex, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import TextEditor from 'shared/components/TextEditor';

import { ViewBotProps } from 'teleport/Bots/types';
import useTeleport from 'teleport/useTeleport';

import { getWorkflowExampleYaml } from './Add/GitHubActions/AddBotToWorkflow';

export function ViewBot({ bot, onClose }: ViewBotProps) {
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  const yaml = getWorkflowExampleYaml(
    bot.name,
    cluster.authVersion,
    cluster.publicURL,
    bot.name,
    false
  );

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>{bot.name}</DialogTitle>
      </DialogHeader>
      <DialogContent width="640px">
        <Text mb="4">
          Below is an example GitHub Actions workflow to help you get started.
          You can find this again from the bot’s options dropdown.
        </Text>
        <Flex height="500px" pt="3" pr="3" bg="levels.deep" borderRadius={3}>
          <TextEditor
            readOnly={true}
            bg="levels.deep"
            data={[{ content: yaml, type: 'yaml' }]}
            copyButton={true}
            downloadButton={true}
            downloadFileName={`${bot.name}-githubactions.yaml`}
          />
        </Flex>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
