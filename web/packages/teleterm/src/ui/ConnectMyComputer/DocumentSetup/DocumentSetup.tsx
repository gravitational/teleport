/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useState } from 'react';
import { Box, ButtonPrimary, Text } from 'design';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';

interface DocumentConnectMyComputerSetupProps {
  visible: boolean;
  doc: types.DocumentConnectMyComputerSetup;
}

export function DocumentSetup(props: DocumentConnectMyComputerSetupProps) {
  const [step, setStep] =
    useState<'information' | 'agent-setup'>('information');

  return (
    <Document visible={props.visible}>
      <Box maxWidth="590px" mx="auto" mt="4" px="5" width="100%">
        <Text typography="h3" mb="4">
          Connect My Computer
        </Text>
        {step === 'information' && (
          <Information onSetUpAgentClick={() => setStep('agent-setup')} />
        )}
        {step === 'agent-setup' && <AgentSetup />}
      </Box>
    </Document>
  );
}

function Information(props: { onSetUpAgentClick(): void }) {
  return (
    <>
      {/* TODO(gzdunek): change the temporary copy */}
      <Text>
        The setup process will download the teleport agent and configure it to
        make your computer available in the <strong>acme.teleport.sh</strong>{' '}
        cluster.
        <br />
        All cluster users with the role{' '}
        <strong>connect-my-computer-robert@acme.com</strong> will be able to
        access your computer as <strong>bob</strong>.
        <br />
        You can stop computer sharing at any time from Connect My Computer menu.
      </Text>
      <ButtonPrimary
        mt={4}
        mx="auto"
        css={`
          display: block;
        `}
        onClick={props.onSetUpAgentClick}
      >
        Set up agent
      </ButtonPrimary>
    </>
  );
}

function AgentSetup() {
  const steps = [
    'Setting up roles',
    'Downloading the agent',
    'Generating the config file',
    'Joining the cluster',
  ];

  return (
    <ol
      css={`
        padding-left: 0;
        list-style: inside decimal;
      `}
    >
      {steps.map(step => (
        <li key={step}>{step}</li>
      ))}
    </ol>
  );
}
