import styled from 'styled-components';

import { ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Flex from 'design/Flex';
import { getPlatform, Platform } from 'design/platform';
import { H3 } from 'design/Text';

const ShortcutsGrid = styled.div`
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 16px 32px;
  align-items: center;
`;

const ShortcutLabel = styled.div`
  font-size: 14px;
  color: ${p => p.theme.colors.text.main};
`;

const ShortcutKeysContainer = styled.div`
  display: flex;
  height: 100%;
  align-items: flex-start;
  justify-content: flex-end;
  padding-top: 2px;
`;

const CmdKey = styled.span`
  font-family: ${p => p.theme.fonts.mono};
  font-size: ${p => p.theme.fontSizes[3]}px;
  position: relative;
  top: 1px;
`;

const ShortcutKeys = styled.div`
  display: inline-flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
  color: ${p => p.theme.colors.text.main};
  background: ${p => p.theme.colors.spotBackground[0]};
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: ${p => p.theme.radii[2]}px;
  padding: 4px 8px;
  line-height: 1;
  font-family: ${p => p.theme.fonts.mono};
  font-size: ${p => p.theme.fontSizes[1]}px;
`;

const ShortcutLabelHint = styled.div`
  font-size: 12px;
  color: ${p => p.theme.colors.text.slightlyMuted};
  margin-top: 4px;
`;

interface KeyboardShortcutsProps {
  open: boolean;
  onClose: () => void;
}

const platform = getPlatform();

export function KeyboardShortcuts({ open, onClose }: KeyboardShortcutsProps) {
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
        height: '100%',
        maxHeight: '580px',
      })}
      disableEscapeKeyDown={false}
      open={open}
      onClose={onClose}
    >
      <DialogHeader>
        <DialogTitle>Keyboard Shortcuts</DialogTitle>
      </DialogHeader>
      <DialogContent overflow={'auto'}>
        <Flex flexDirection="column" gap={4} mb={3}>
          <ShortcutsGrid>
            <ShortcutLabel>Show Keyboard Shortcuts</ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>?</ShortcutKeys>
            </ShortcutKeysContainer>

            <ShortcutLabel>Show/Hide Timeline</ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>t</ShortcutKeys>
            </ShortcutKeysContainer>

            <ShortcutLabel>Show/Hide Sidebar</ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>s</ShortcutKeys>
            </ShortcutKeysContainer>

            <ShortcutLabel>
              Toggle absolute time
              <ShortcutLabelHint>
                Will replace the minute markers with the time of day
              </ShortcutLabelHint>
            </ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>a</ShortcutKeys>
            </ShortcutKeysContainer>
          </ShortcutsGrid>

          <DialogTitle>Interactivity</DialogTitle>

          <ShortcutsGrid>
            <ShortcutLabel>Zoom</ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>
                {platform === Platform.macOS ? (
                  <>
                    <CmdKey>⌘</CmdKey> + scroll
                  </>
                ) : (
                  'Ctrl + scroll'
                )}
              </ShortcutKeys>
            </ShortcutKeysContainer>

            <ShortcutLabel>Pan/Scroll</ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>shift + scroll</ShortcutKeys>
            </ShortcutKeysContainer>
          </ShortcutsGrid>

          <H3>Trackpad Interactivity</H3>

          <ShortcutsGrid>
            <ShortcutLabel>Zoom</ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>
                <CmdKey>⌘</CmdKey> + two finger up/down
              </ShortcutKeys>
            </ShortcutKeysContainer>

            <ShortcutLabel>Pan/Scroll</ShortcutLabel>

            <ShortcutKeysContainer>
              <ShortcutKeys>Two finger left/right</ShortcutKeys>
            </ShortcutKeysContainer>
          </ShortcutsGrid>
        </Flex>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close (esc)</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
