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
import { Text } from 'design';

import styled from 'styled-components';

import Document from 'teleterm/ui/Document';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';
import { KeyboardShortcutAction } from 'teleterm/services/config';

export function KeyboardShortcutsPanel() {
  const { getAccelerator } = useKeyboardShortcutFormatters();

  const items: { title: string; shortcutAction: KeyboardShortcutAction }[] = [
    {
      title: 'Open New Tab',
      shortcutAction: 'newTab',
    },
    {
      title: 'Go To Next Tab',
      shortcutAction: 'nextTab',
    },
    {
      title: 'Open Connections',
      shortcutAction: 'openConnections',
    },
    {
      title: 'Open Clusters',
      shortcutAction: 'openClusters',
    },
    {
      title: 'Open Profiles',
      shortcutAction: 'openProfiles',
    },
  ];

  return (
    <Document visible={true}>
      <Grid>
        {items.map(item => (
          <Entry
            title={item.title}
            accelerator={getAccelerator(item.shortcutAction, {
              useWhitespaceSeparator: true,
            })}
            key={item.shortcutAction}
          />
        ))}
      </Grid>
    </Document>
  );
}

function Entry(props: { title: string; accelerator: string }) {
  return (
    <>
      <Text textAlign="right" color="light" typography="subtitle1" py="4px">
        {props.title}
      </Text>
      <MonoText
        bg="levels.surfaceSecondary"
        textAlign="left"
        px="12px"
        py="4px"
      >
        {props.accelerator}
      </MonoText>
    </>
  );
}

const MonoText = styled(Text)`
  font-family: ${props => props.theme.fonts.mono};
  width: fit-content;
  opacity: 0.7;
  border-radius: 4px;
`;

const Grid = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  align-items: end;
  column-gap: 32px;
  row-gap: 14px;
  margin: auto;
`;
