/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useEffect, useState } from 'react';
import {
  Box,
  Link,
  Text,
  Alert,
  Flex,
  ButtonPrimary,
  ButtonSecondary,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogTitle,
} from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import useTeleport from 'teleport/useTeleport';

import useAddKube, { State } from './useAddKube';

export default function Container(props: Props) {
  const ctx = useTeleport();
  const state = useAddKube(ctx);
  return <AddKube {...state} {...props} />;
}

export function AddKube({
  onClose,
  attempt,
  createToken,
  token,
  version,
}: Props & State) {
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;

  const [cmd, setCmd] = useState('');
  const [namespace, setNamespace] = useState('');
  const [clusterName, setClusterName] = useState('');

  useEffect(() => {
    if (!token) {
      setCmd('');
      return;
    }

    const generatedCmd = generateCmd(
      namespace,
      clusterName,
      host,
      token.id,
      version
    );
    setCmd(generatedCmd);
  }, [token]);

  function handleSubmit(e: React.SyntheticEvent, validator: Validator) {
    e.preventDefault();

    // validate() will run the rule functions of the form inputs
    // it will automatically update the UI with error messages if the validation fails.
    // No need for further actions here in case it fails
    if (!validator.validate()) {
      return;
    }

    createToken();
  }

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '600px',
        width: '100%',
        minHeight: '328px',
      })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <Flex flex="1" flexDirection="column">
        <DialogTitle mr="auto" mb="4">
          Add Kubernetes
        </DialogTitle>
        {attempt.status == 'failed' && (
          <Alert kind="danger" children={attempt.statusText} />
        )}
        <DialogContent minHeight="254px" flex="0 0 auto" mb="2">
          <Box mb={4}>
            Install Teleport Agent in your cluster via Helm to easily connect
            your Kubernetes cluster with Teleport. For all the available values
            of the helm chart see{' '}
            <Link
              href="https://goteleport.com/docs/kubernetes-access/helm/reference/teleport-kube-agent/"
              target="_blank"
            >
              the documentation
            </Link>
            {'.'}
          </Box>
          <Box mb={4}>
            <Text>
              <Text bold as="span">
                Step 1
              </Text>
              {' - Add teleport-agent chart to your charts repository'}
            </Text>
            <TextSelectCopy
              text={
                'helm repo add teleport https://charts.releases.teleport.dev && helm repo update'
              }
            />
          </Box>
          <Box mb={4}>
            <Text bold as="span">
              Step 2
            </Text>
            {
              ' - Generate a script to automatically configure and install the teleport-agent'
            }
            <Validation>
              {({ validator }) => (
                <Flex alignItems="center" flexDirection="row">
                  <form
                    onSubmit={e => handleSubmit(e, validator)}
                    style={{ width: '100%' }}
                  >
                    <FieldInput
                      mb={2}
                      rule={requiredField('Namespace is required')}
                      label="Namespace"
                      autoFocus
                      value={namespace}
                      placeholder="teleport"
                      width="100%"
                      mr="3"
                      onChange={e => setNamespace(e.target.value)}
                    />
                    <FieldInput
                      mb={2}
                      rule={requiredField(
                        'Kubernetes Cluster Name is required'
                      )}
                      label="Kubernetes Cluster Name"
                      labelTip="Name shown to Teleport users connecting to the cluster."
                      value={clusterName}
                      placeholder="my-cluster"
                      width="100%"
                      mr="3"
                      onChange={e => setClusterName(e.target.value)}
                    />
                    <ButtonPrimary
                      block
                      mt="2"
                      disabled={attempt.status === 'processing'}
                      type="submit"
                    >
                      {cmd ? 'Regenerate Script' : 'Generate Script'}
                    </ButtonPrimary>
                  </form>
                </Flex>
              )}
            </Validation>
          </Box>
          {cmd && (
            <Box mb={4}>
              <Text bold as="span">
                Step 3
              </Text>
              {' - Install the helm chart'}

              <Box>
                <Text mt="2" mb="1">
                  The token will be valid for{' '}
                  <Text bold as={'span'}>
                    {token.expiryText}.
                  </Text>
                </Text>
                <TextSelectCopy text={cmd} mb={2} />
                <Text>
                  <Text as="span" bold>
                    Tip
                  </Text>
                  : Save the YAML file to apply updates later
                </Text>
              </Box>
            </Box>
          )}
        </DialogContent>
        <DialogFooter>
          <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
        </DialogFooter>
      </Flex>
    </Dialog>
  );
}
const generateCmd = (
  namespace: string,
  clusterName: string,
  proxyAddr: string,
  tokenId: string,
  clusterVersion: string
) => {
  return `cat << EOF > prod-cluster-values.yaml
roles: kube
authToken: ${tokenId}
proxyAddr: ${proxyAddr}
kubeClusterName: ${clusterName}
teleportVersionOverride: ${clusterVersion}
EOF
 
helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --create-namespace --namespace ${namespace}`;
};

export type Props = {
  onClose(): void;
};
