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

import React, { useState } from 'react';
import Dialog, {
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogContent,
} from 'design/Dialog';
import Validation from 'shared/components/Validation';
import {
  Text,
  Box,
  ButtonSecondary,
  ButtonPrimary,
  Flex,
  Toggle,
} from 'design';

import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { generateTshLoginCommand, openNewTab } from 'teleport/lib/util';
import { AuthType } from 'teleport/services/user';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';

function ConnectDialog(props: Props) {
  const {
    onClose,
    username,
    authType,
    kubeConnectName,
    clusterId,
    accessRequestId,
  } = props;

  const [execPath, setExecPath] = useState('');
  const [execCommand, setExecCommand] = useState('');
  const [execInteractive, setExecInteractive] = useState(true);

  const startKubeExecSession = () => {
    const splitPath = execPath.split('/');

    const url = cfg.getKubeExecConnectRoute(
      {
        clusterId,
        kubeId: kubeConnectName,
        namespace: splitPath[0],
        pod: splitPath[1],
        container: splitPath?.[2],
      },
      { isInteractive: execInteractive, command: execCommand }
    );

    openNewTab(url);
  };

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>connect to kubernetes cluster</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Box mb={4}>
          <Text bold as="span">
            Step 1
          </Text>
          {' - Login to Teleport'}
          <TextSelectCopy
            mt="2"
            text={generateTshLoginCommand({
              authType,
              username,
              clusterId,
              accessRequestId,
            })}
          />
        </Box>
        <Box mb={4}>
          <Text bold as="span">
            Optional
          </Text>{' '}
          - To write kubectl configuration to a separate file instead of having
          your global kubectl configuration modified, run the following command:
          <TextSelectCopy
            mt="2"
            text="export KUBECONFIG=${HOME?}/teleport-kubeconfig.yaml"
          />
        </Box>
        <Box mb={4}>
          <Text bold as="span">
            Step 2
          </Text>
          {' - Select the Kubernetes cluster'}
          <TextSelectCopy mt="2" text={`tsh kube login ${kubeConnectName}`} />
        </Box>
        <Box mb={1}>
          <Text bold as="span">
            Step 3
          </Text>
          {' - Connect to the Kubernetes cluster'}
          <TextSelectCopy mt="2" text={`kubectl get pods`} />
        </Box>
        {accessRequestId && (
          <Box mb={1} mt={3}>
            <Text bold as="span">
              Step 4 (Optional)
            </Text>
            {' - When finished, drop the assumed role'}
            <TextSelectCopy mt="2" text={`tsh request drop`} />
          </Box>
        )}
        <Validation>
          {() => (
            <Box borderTop={1} mb={4} mt={4}>
              <Text mt={3} bold>
                Or exec into a pod on this Kubernetes cluster
              </Text>
              <Flex gap={3}>
                <FieldInput
                  value={execPath}
                  placeholder="namespace/pod"
                  label="Pod to exec into"
                  width="50%"
                  onChange={e => setExecPath(e.target.value.trim())}
                  toolTipContent={
                    <Text>
                      Specify namespace and pod you want to exec into.
                      Optionally you can also specify container by adding it at
                      the end: '/namespace/pod/container'
                    </Text>
                  }
                />
                <FieldInput
                  rule={requiredField('Command to execute is required')}
                  value={execCommand}
                  placeholder="/bin/bash"
                  label="Command to execute"
                  width="50%"
                  onChange={e => setExecCommand(e.target.value)}
                  toolTipContent={
                    <Text>
                      The command that will be executed inside the target pod.
                    </Text>
                  }
                />
              </Flex>
              <Flex justifyContent="space-between" gap={3}>
                <Toggle
                  isToggled={execInteractive}
                  onToggle={() => {
                    setExecInteractive(b => !b);
                  }}
                >
                  <Box ml={2} mr={1}>
                    Interactive shell
                  </Box>
                  <ToolTipInfo>
                    You can start an interactive shell and have a bidirectional
                    communication with the target pod, or you can run one-off
                    command and see its output.
                  </ToolTipInfo>
                </Toggle>
                <ButtonPrimary onClick={startKubeExecSession}>
                  Run Command
                </ButtonPrimary>
              </Flex>
            </Box>
          )}
        </Validation>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  onClose: () => void;
  username: string;
  authType: AuthType;
  kubeConnectName: string;
  clusterId: string;
  accessRequestId?: string;
};

const dialogCss = () => `
  min-height: 400px;
  max-width: 600px;
  width: 100%;
`;

export default ConnectDialog;
