import React from 'react';
import { Text } from 'design';
import Document from 'teleterm/ui/Document';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import styled from 'styled-components';
import { Platform } from 'teleterm/mainProcess/types';

export function KeyboardShortcutsPanel() {
  const ctx = useAppContext();
  const { keyboardShortcuts } = ctx.mainProcessClient.configService.get();

  const items = [
    {
      title: 'Open New Tab',
      shortcut: keyboardShortcuts['tab-new'],
    },
    {
      title: 'Go To Next Tab',
      shortcut: keyboardShortcuts['tab-next'],
    },
    {
      title: 'Open Connections',
      shortcut: keyboardShortcuts['toggle-connections'],
    },
    {
      title: 'Open Clusters',
      shortcut: keyboardShortcuts['toggle-clusters'],
    },
    {
      title: 'Open Profile',
      shortcut: keyboardShortcuts['toggle-identity'],
    },
  ];

  return (
    <Document visible={true}>
      <Grid>
        {items.map(item => (
          <Entry
            title={item.title}
            shortcut={displayShortcut(
              ctx.mainProcessClient.getRuntimeSettings().platform,
              item.shortcut
            )}
            key={item.shortcut}
          />
        ))}
      </Grid>
    </Document>
  );
}

function Entry(props: { title: string; shortcut: string }) {
  return (
    <>
      <Text textAlign="right" color="light" typography="subtitle1" py="4px">
        {props.title}
      </Text>
      <MonoText bg="primary.lighter" textAlign="left" px="12px" py="4px">
        {props.shortcut}
      </MonoText>
    </>
  );
}

function displayShortcut(platform: Platform, shortcut: string): string {
  switch (platform) {
    case 'darwin':
      return shortcut
        .replace('-', ' ')
        .replace('Command', '⌘')
        .replace('Control', '⌃')
        .replace('Option', '⌥')
        .replace('Shift', '⇧');
    case 'linux':
      return shortcut.replace('-', ' + ');
  }
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
