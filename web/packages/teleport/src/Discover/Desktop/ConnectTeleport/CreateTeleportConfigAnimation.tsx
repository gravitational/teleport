import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Editor, File, Language } from 'shared/components/Editor';

import { useJoinTokenValue } from 'teleport/Discover/Shared/JoinTokenContext';

import type { JoinToken } from 'teleport/services/joinToken';

const pastedLines = (joinToken: JoinToken) => `version: v3
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
    teleport.internal/resource-id: ${joinToken.internalResourceId}`;

enum EditorState {
  Original,
  Pasted,
}

const states = (joinToken: JoinToken) => [
  {
    kind: EditorState.Original,
    content: '',
  },
  {
    kind: EditorState.Pasted,
    content: pastedLines(joinToken),
  },
];

export function CreateTeleportConfigAnimation() {
  const joinToken = useJoinTokenValue();

  const [editorState, setEditorState] = useState(EditorState.Original);

  const { content } = states(joinToken).find(
    state => state.kind === editorState
  );

  useEffect(() => {
    setEditorState(EditorState.Original);

    const id = window.setTimeout(
      () => setEditorState(EditorState.Pasted),
      1500
    );

    return () => clearTimeout(id);
  }, []);

  return (
    <DisableUserSelect>
      <Editor title="Your IDE">
        <File
          language={Language.YAML}
          name="/etc/teleport.yaml"
          code={content}
        />
      </Editor>
    </DisableUserSelect>
  );
}

const DisableUserSelect = styled('div')`
  user-select: none;
`;
