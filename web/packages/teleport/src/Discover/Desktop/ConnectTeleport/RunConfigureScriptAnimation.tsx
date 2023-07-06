/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import {
  AnimatedTerminal,
  TerminalColor,
} from 'shared/components/AnimatedTerminal';

import { KeywordHighlight } from 'shared/components/AnimatedTerminal/TerminalContent';

import { ResourceKind } from 'teleport/Discover/Shared';

import { generateCommand } from 'teleport/Discover/Shared/generateCommand';
import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { JoinToken } from 'teleport/services/joinToken';

const lines = (joinToken: JoinToken) => [
  {
    text: generateCommand(
      'https://teleport.example.com/webapi/scripts/desktop-access/configure/<YOUR_TOKEN>/configure-ad.ps1'
    ),
    isCommand: true,
  },
  {
    text: 'Running...',
    isCommand: false,
    delay: 800,
  },
  {
    text: `
version: v3
teleport:
  auth_token: ${joinToken.id}
  proxy_server: ${window.location.hostname}:${window.location.port || '443'}

auth_service:
  enabled: no
ssh_service:
  enabled: no
proxy_service:
  enabled: no

windows_desktop_service:
  enabled: yes
  ldap:
    addr:        127.0.0.1:636
    domain:      TELEPORT
    username:    example
    server_name: desktop.teleport.example
    insecure_skip_verify: false
    ldap_ca_cert: |
      -----THIS IS JUST AN EXAMPLE-----
  discovery:
    base_dn: '*'
  labels:
    teleport.internal/resource-id: ${joinToken.internalResourceId}
`,
    isCommand: false,
    delay: 500,
  },
  {
    text: 'Desktop Access Configuration Reference: https://goteleport.com/docs/desktop-access/reference/configuration/',
    isCommand: false,
    delay: 500,
  },
  {
    text: '',
    isCommand: true,
  },
];

const selectedLines = {
  start: 3,
  end: 28,
};

const highlights: KeywordHighlight[] = [
  {
    key: 'keyword',
    color: TerminalColor.Keyword,
    keywords: ['Invoke-WebRequest', 'Invoke-Expression'],
  },
  {
    key: 'arg',
    color: TerminalColor.Argument,
    keywords: ['-Uri'],
  },
];

interface RunConfigureScriptAnimationProps {
  isCopying: boolean;
}

export function RunConfigureScriptAnimation(
  props: RunConfigureScriptAnimationProps
) {
  const { joinToken } = useJoinTokenSuspender(ResourceKind.Desktop);

  return (
    <AnimatedTerminal
      lines={lines(joinToken)}
      highlights={highlights}
      selectedLines={props.isCopying ? selectedLines : null}
      stopped={props.isCopying}
    />
  );
}
