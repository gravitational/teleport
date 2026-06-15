import { screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import {
  MOCK_DESKTOP_EVENTS,
  MOCK_EVENTS,
} from 'e-teleport/SessionRecordings/summary/mock';
import { SessionRecordingTimeline } from 'e-teleport/SessionRecordings/summary/SessionRecordingTimeline';

describe('SessionRecordingTimeline', () => {
  it('renders desktop events alongside command events', () => {
    render(
      <SessionRecordingTimeline
        events={[...MOCK_EVENTS, ...MOCK_DESKTOP_EVENTS]}
        onPlay={() => null}
        inferenceDuration={null}
        sessionDuration={30_000}
      />
    );

    expect(screen.getByText('Remote Script Execution')).toBeInTheDocument();
    expect(
      screen.getByText('Uploaded Payroll Data to Personal Cloud Storage')
    ).toBeInTheDocument();
    expect(
      screen.getByText('Disabled Real-Time Antivirus Protection')
    ).toBeInTheDocument();
  });
});
