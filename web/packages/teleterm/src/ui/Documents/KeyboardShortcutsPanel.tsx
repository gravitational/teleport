import React from 'react';
import { Text } from 'design';

import styled from 'styled-components';

import Document from 'teleterm/ui/Document';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';
import { KeyboardShortcutType } from 'teleterm/services/config';

export function KeyboardShortcutsPanel() {
  const { getShortcut } = useKeyboardShortcutFormatters();

  const items: { title: string; shortcutKey: KeyboardShortcutType }[] = [
    {
      title: 'Open New Tab',
      shortcutKey: 'tab-new',
    },
    {
      title: 'Go To Next Tab',
      shortcutKey: 'tab-next',
    },
    {
      title: 'Open Connections',
      shortcutKey: 'toggle-connections',
    },
    {
      title: 'Open Clusters',
      shortcutKey: 'toggle-clusters',
    },
    {
      title: 'Open Profiles',
      shortcutKey: 'toggle-identity',
    },
  ];

  return (
    <Document visible={true}>
      <Grid>
        {items.map(item => (
          <Entry
            title={item.title}
            shortcut={getShortcut(item.shortcutKey, {
              useWhitespaceSeparator: true,
            })}
            key={item.shortcutKey}
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
      <MonoText bg="primary.main" textAlign="left" px="12px" py="4px">
        {props.shortcut}
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
