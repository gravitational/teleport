import type { StoryObj } from '@storybook/react-vite';
import { intervalToDuration } from 'date-fns';

import { MOCK_EVENTS } from 'e-teleport/SessionRecordings/summary/mock';
import { SessionRecordingTimeline } from 'e-teleport/SessionRecordings/summary/SessionRecordingTimeline';

export default {
  title: 'TeleportE/SessionRecordings',
};

export const Timeline: StoryObj = {
  name: 'Session Recording Timeline',
  render: () => (
    <SessionRecordingTimeline
      events={MOCK_EVENTS}
      onPlay={() => null}
      inferenceDuration={intervalToDuration({
        start: new Date(),
        end: new Date(Date.now() + 60000),
      })}
      sessionDuration={10_000}
    />
  ),
};
