import React from 'react';

import {
  AnimatedTerminal,
  TerminalColor,
} from 'shared/components/AnimatedTerminal';

import { KeywordHighlight } from 'shared/components/AnimatedTerminal/TerminalContent';

import cfg from 'teleport/config';
import { ResourceKind } from 'teleport/Discover/Shared';

import { generateCommand } from 'teleport/Discover/Shared/generateCommand';
import { useJoinToken } from 'teleport/Discover/Shared/JoinTokenContext';
import { JoinToken } from 'teleport/services/joinToken';

const lines = (joinToken: JoinToken) => [
  {
    text: generateCommand(cfg.getConfigureADUrl(joinToken.id)),
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
  start: 4,
  end: 29,
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
  const { joinToken } = useJoinToken(ResourceKind.Desktop);

  return (
    <AnimatedTerminal
      lines={lines(joinToken)}
      highlights={highlights}
      selectedLines={props.isCopying ? selectedLines : null}
      stopped={props.isCopying}
    />
  );
}
