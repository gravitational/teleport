/**
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

import { useState } from 'react';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Text,
  Toggle,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { IconTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

type Props = {
  onClose(): void;
  onExec(
    namespace: string,
    pod: string,
    container: string,
    command: string,
    isInteractive: boolean
  ): void;
};

function KubeExecDataDialog({ onClose, onExec }: Props) {
  const [namespace, setNamespace] = useState('');
  const [pod, setPod] = useState('');
  const [container, setContainer] = useState('');
  const [execCommand, setExecCommand] = useState('');
  const [execInteractive, setExecInteractive] = useState(true);

  const podExec = () => {
    onExec(namespace, pod, container, execCommand, execInteractive);
  };

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <Validation>
        {({ validator }) => (
          <form>
            <DialogHeader>
              <DialogTitle>Exec into a pod</DialogTitle>
            </DialogHeader>
            <DialogContent>
              <Box mb={4}>
                <Flex gap={3}>
                  <FieldInput
                    value={namespace}
                    placeholder="namespace"
                    label="Namespace"
                    autoFocus
                    rule={requiredField('Namespace is required')}
                    width="33%"
                    onChange={e => setNamespace(e.target.value.trim())}
                  />
                  <FieldInput
                    value={pod}
                    placeholder="pod"
                    label="Pod"
                    rule={requiredField('Pod is required')}
                    width="33%"
                    onChange={e => setPod(e.target.value.trim())}
                  />
                  <FieldInput
                    value={container}
                    placeholder="container"
                    label="Container (optional)"
                    width="33%"
                    onChange={e => setContainer(e.target.value.trim())}
                  />
                </Flex>
                <Flex justifyContent="space-between" gap={3}>
                  <FieldInput
                    rule={requiredField('Command to execute is required')}
                    value={execCommand}
                    placeholder="/bin/bash"
                    label="Command to execute"
                    width="66%"
                    onChange={e => setExecCommand(e.target.value)}
                    toolTipContent={
                      <Text>
                        The command that will be executed inside the target pod.
                      </Text>
                    }
                  />
                  <Toggle
                    isToggled={execInteractive}
                    onToggle={() => {
                      setExecInteractive(b => !b);
                    }}
                  >
                    <Box ml={2} mr={1}>
                      Interactive shell
                    </Box>
                    <IconTooltip>
                      You can start an interactive shell and have a
                      bidirectional communication with the target pod, or you
                      can run one-off command and see its output.
                    </IconTooltip>
                  </Toggle>
                </Flex>
              </Box>
            </DialogContent>
            <DialogFooter>
              <Flex justifyContent="space-between">
                <ButtonSecondary
                  type="button"
                  width="45%"
                  size="large"
                  onClick={onClose}
                >
                  Close
                </ButtonSecondary>
                <ButtonPrimary
                  type="submit"
                  width="45%"
                  size="large"
                  onClick={e => {
                    e.preventDefault();
                    validator.validate() && podExec();
                  }}
                >
                  Exec
                </ButtonPrimary>
              </Flex>
            </DialogFooter>
          </form>
        )}
      </Validation>
    </Dialog>
  );
}

const dialogCss = () => `
  min-height: 200px;
  max-width: 600px;
  width: 100%;
`;

export default KubeExecDataDialog;
