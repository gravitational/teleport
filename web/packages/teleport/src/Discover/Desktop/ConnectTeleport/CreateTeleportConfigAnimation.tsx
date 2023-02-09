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

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Editor, File, Language } from 'shared/components/Editor';

import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { ResourceKind } from 'teleport/Discover/Shared';

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
  const { joinToken } = useJoinTokenSuspender(ResourceKind.Desktop);

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
