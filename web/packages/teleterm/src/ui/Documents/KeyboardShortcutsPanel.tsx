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

import styled from 'styled-components';

import { Text } from 'design';

import { KeyboardShortcutAction } from 'teleterm/services/config';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';

export function KeyboardShortcutsPanel() {
  const { mainProcessClient } = useAppContext();
  const { platform } = mainProcessClient.getRuntimeSettings();
  const { getAccelerator } = useKeyboardShortcutFormatters();
  const isNotMac = platform !== 'darwin';

  const items: { title: string; shortcutAction: KeyboardShortcutAction }[] = [
    {
      title: 'Open New Tab',
      shortcutAction: 'newTab',
    },
    {
      title: 'Open New Terminal Tab',
      shortcutAction: 'newTerminalTab',
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
    // We don't need to show these shortcuts on macOS,
    // 99% of the users are not going to change them.
    isNotMac && {
      title: 'Copy in Terminal',
      shortcutAction: 'terminalCopy',
    },
    isNotMac && {
      title: 'Paste in Terminal',
      shortcutAction: 'terminalPaste',
    },
  ];

  return (
    <Document visible={true}>
      <Grid>
        {items.filter(Boolean).map(item => (
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
      <Text textAlign="right" typography="body2" py="4px">
        {props.title}
      </Text>
      <MonoText
        css={`
          background: ${props => props.theme.colors.spotBackground[0]};
        `}
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
  width: fit-content;
  border-radius: 4px;
`;

const Grid = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  align-items: end;
  column-gap: ${props => props.theme.space[4]}px;
  row-gap: ${props => props.theme.space[3]}px;
  margin: auto;
  padding-block: ${props => props.theme.space[3]}px;
`;
