import { useState } from 'react';
import { render, screen, fireEvent } from 'design/utils/testing';
import { Option } from 'shared/components/Select';

import { dryRunResponse } from 'e-teleport/Workflow/fixtures';

import { Start } from '../Shared/types';

import { AssumeStartTime } from './AssumeStartTime';

test('assume start time', () => {
  jest.useFakeTimers().setSystemTime(dryRunResponse.created);
  render(<AssumeStartTimeComp />);

  // Init state.
  expect(screen.queryByText(/start time/i)).not.toBeInTheDocument();
  expect(screen.getByText(/access duration/i)).toBeInTheDocument();
  expect(screen.getAllByText(/2 days/i)).toHaveLength(1);
  const calendarBtn = screen.getByText(/immediately/i);
  fireEvent.click(calendarBtn);

  // Selecting a date on the date picker should generate a
  // "time" and "access duration" dropdown.
  fireEvent.click(screen.getByText(/15/i));
  expect(screen.queryByText(/immediately/i)).not.toBeInTheDocument();
  expect(screen.getByText(/february 15, 2024/i)).toBeInTheDocument();

  expect(screen.getByText(/start time/i)).toBeInTheDocument();
  expect(screen.getByText(/3:00 AM/i)).toBeInTheDocument();

  expect(
    screen.getByText('1 day 23 hours 51 minutes (Max Duration)')
  ).toBeInTheDocument();

  // Selecting a different start "time" should change the
  // "access duration" time.
  const timeOptBox = screen.getByText(/3:00 AM/i);
  fireEvent.keyDown(timeOptBox, { key: 'ArrowDown' });
  fireEvent.click(screen.getByText(/11:00 PM/i)); // 10 hour later

  expect(screen.getByText(/11:00 PM/i)).toBeInTheDocument();
  expect(
    screen.getByText('1 day 3 hours 51 minutes (Max Duration)')
  ).toBeInTheDocument();

  // Clicking "immediately" button goes back to default values.
  fireEvent.click(screen.getByText(/february 15, 2024/i));
  fireEvent.click(screen.getByText(/immediately/i));
  expect(screen.getByText(/immediately/i)).toBeInTheDocument();
  expect(screen.queryByText(/start time/i)).not.toBeInTheDocument();
  expect(screen.getByText(/2 days/i)).toBeInTheDocument();
});

const AssumeStartTimeComp = () => {
  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  const [start, setStart] = useState<Start>();

  return (
    <div>
      <AssumeStartTime
        maxDuration={maxDuration}
        setMaxDuration={setMaxDuration}
        start={start}
        setStart={setStart}
        accessRequest={dryRunResponse}
      />
    </div>
  );
};
