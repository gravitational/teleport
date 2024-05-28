/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useState } from 'react';
import { render, screen, fireEvent } from 'design/utils/testing';

import { Option } from 'shared/components/Select';

import { AccessRequest } from 'shared/services/accessRequests';

import { dryRunResponse } from '../fixtures';
import { AccessDurationRequest, AccessDurationReview } from '../AccessDuration';

import { AssumeStartTime } from './AssumeStartTime';

test('assume start time, creating mode', () => {
  jest.useFakeTimers().setSystemTime(dryRunResponse.created);
  render(<AssumeStartTimeComp accessRequest={dryRunResponse} />);

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
  expect(screen.queryByTestId('reset-btn')).not.toBeInTheDocument();
  expect(screen.getByText(/february 15, 2024/i)).toBeInTheDocument();

  expect(screen.getByText(/start time/i)).toBeInTheDocument();
  expect(screen.getByText(/3:00 AM/i)).toBeInTheDocument();

  expect(screen.getByText('1 day 23 hours 51 minutes')).toBeInTheDocument();

  // Selecting a different start "time" should change the
  // "access duration" time.
  const timeOptBox = screen.getByText(/3:00 AM/i);
  fireEvent.keyDown(timeOptBox, { key: 'ArrowDown' });
  fireEvent.click(screen.getByText(/11:00 PM/i)); // 10 hour later

  expect(screen.getByText(/11:00 PM/i)).toBeInTheDocument();
  expect(screen.getByText('1 day 3 hours 51 minutes')).toBeInTheDocument();

  // Clicking "immediately" button goes back to default values.
  fireEvent.click(screen.getByText(/february 15, 2024/i));
  fireEvent.click(screen.getByText(/immediately/i));
  expect(screen.getByText(/immediately/i)).toBeInTheDocument();
  expect(screen.queryByText(/start time/i)).not.toBeInTheDocument();
  expect(screen.getByText(/2 days/i)).toBeInTheDocument();
});

test('assume start time, reviewing mode, with assume start time', () => {
  const withStart = {
    ...dryRunResponse,
    assumeStartTime: new Date('2024-02-16T02:51:12.70087Z'),
  };
  jest.useFakeTimers().setSystemTime(withStart.created);
  render(<AssumeStartTimeComp accessRequest={withStart} review={true} />);

  // Init state should render the requested assume start date and time.
  expect(screen.getByText(/start date/i)).toBeInTheDocument();
  expect(screen.getByText(/start time/i)).toBeInTheDocument();
  expect(screen.getByText(/requested/i)).toBeInTheDocument();
  expect(screen.getByText(/1 day/i)).toBeInTheDocument();
  expect(screen.getByText(/access duration:/i)).toBeInTheDocument();
  expect(screen.queryByTestId('reset-btn')).not.toBeInTheDocument();

  // Changing time should render reset button and update access duration.
  const timeOptBox = screen.getByText('2:51 AM (Requested)');
  fireEvent.keyDown(timeOptBox, { key: 'ArrowDown' });
  fireEvent.click(screen.getByText(/3:00 AM/i));
  expect(screen.getByTestId('reset-btn')).toBeInTheDocument();
  expect(screen.getByText(/23 hours 51 minutes/i)).toBeInTheDocument();

  // Clicking on reset button should go back to the requested time.
  fireEvent.click(screen.getByTestId('reset-btn'));
  expect(screen.getByText(/1 day/i)).toBeInTheDocument();
  expect(screen.getByText('2:51 AM (Requested)')).toBeInTheDocument();

  // Clicking on "immediately" button, should change time to "now".
  fireEvent.click(screen.getByText(/february 16, 2024/i));
  fireEvent.click(screen.getByText(/immediately/i));
  expect(screen.getByText(/immediately/i)).toBeInTheDocument();
  expect(screen.getByText(/2 days/i)).toBeInTheDocument();
});

test('assume start time, reviewing mode, with NO assume start time', () => {
  jest.useFakeTimers().setSystemTime(dryRunResponse.created);
  render(<AssumeStartTimeComp accessRequest={dryRunResponse} review={true} />);

  // Init state should not render time since it wasn't defined
  expect(screen.getByText(/start date/i)).toBeInTheDocument();
  expect(screen.queryByText(/start time/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/requested/i)).not.toBeInTheDocument();
  expect(screen.getByText(/2 days/i)).toBeInTheDocument();
  expect(screen.getByText(/access duration:/i)).toBeInTheDocument();
  expect(screen.queryByTestId('reset-btn')).not.toBeInTheDocument();

  // Clicking on a different date should render time options.
  fireEvent.click(screen.getByText(/immediately/i));
  fireEvent.click(screen.getByText(/16/i));
  expect(screen.getByText(/start time/i)).toBeInTheDocument();
});

const AssumeStartTimeComp = ({
  accessRequest,
  review = false,
}: {
  accessRequest: AccessRequest;
  review?: boolean;
}) => {
  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  const [start, setStart] = useState<Date>();

  if (review) {
    return (
      <>
        <AssumeStartTime
          start={start}
          onStartChange={setStart}
          accessRequest={accessRequest}
          reviewing={true}
        />
        <AccessDurationReview
          assumeStartTime={start}
          accessRequest={accessRequest}
        />
      </>
    );
  }

  return (
    <>
      <AssumeStartTime
        start={start}
        onStartChange={setStart}
        accessRequest={accessRequest}
      />
      <AccessDurationRequest
        assumeStartTime={start}
        accessRequest={accessRequest}
        maxDuration={maxDuration}
        setMaxDuration={setMaxDuration}
      />
    </>
  );
};
