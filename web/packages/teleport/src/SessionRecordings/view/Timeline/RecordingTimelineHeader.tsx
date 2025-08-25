import styled from 'styled-components';

import Flex from 'design/Flex';
import { Cross, Keyboard } from 'design/Icon';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';

const Container = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: var(--header-height);
  box-sizing: border-box;
  background: ${p => p.theme.colors.sessionRecordingTimeline.headerBackground};
  color: ${p => p.theme.colors.text.muted};
  border-bottom: 1px solid
    ${p => p.theme.colors.sessionRecordingTimeline.border.default};
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 calc(${p => p.theme.space[2]}px + ${p => p.theme.space[1]}px);
`;

const HeaderButton = styled.button`
  background: transparent;
  border: none;
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p => p.theme.space[1]}px;
  margin-left: auto;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0.6;

  &:hover {
    opacity: 1;
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

interface RecordingTimelineHeaderProps {
  onHide: () => void;
  onOpenKeyboardShortcuts: () => void;
}

export function RecordingTimelineHeader({
  onHide,
  onOpenKeyboardShortcuts,
}: RecordingTimelineHeaderProps) {
  return (
    <Container>
      <Text fontWeight="500" fontSize="small">
        Session Timeline
      </Text>

      <Flex alignItems="center" gap={2}>
        <HoverTooltip tipContent="Keyboard Shortcuts">
          <HeaderButton onClick={onOpenKeyboardShortcuts}>
            <Keyboard size="small" />
          </HeaderButton>
        </HoverTooltip>

        <HoverTooltip tipContent="Hide Timeline">
          <HeaderButton onClick={onHide}>
            <Cross size="small" />
          </HeaderButton>
        </HoverTooltip>
      </Flex>
    </Container>
  );
}
