/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { fireEvent, render, screen } from 'design/utils/testing';
import Validation from 'shared/components/Validation';

import { getFormattedDurationTxt } from '../../Shared/utils';
import { ScheduledAccessTiming } from './ScheduledAccessTiming';

afterEach(() => {
  jest.useRealTimers();
});

test('immediate layout renders start, duration and the resulting end', () => {
  // Setup an immediate request whose access ends at a known date and time.
  jest.useFakeTimers().setSystemTime(new Date('2026-10-08T08:00:00Z'));
  const end = new Date('2026-10-08T16:30:00Z');

  // Execute.
  render(
    <Validation>
      <ScheduledAccessTiming
        start={null}
        maxAllowedDuration={8 * 60 * 60 * 1000}
        durationOptions={[{ value: end.getTime(), label: '8 hours' }]}
        selectedAccessEnd={{ value: end.getTime(), label: '8 hours' }}
        onStartChange={jest.fn()}
        onAccessEndChange={jest.fn()}
      />
    </Validation>
  );

  // Assert the start control reads as immediate.
  expect(screen.getByText('Access starts')).toBeVisible();
  expect(screen.getByText('Immediately on Approval')).toBeVisible();

  // Assert the duration selector shows the selected duration.
  expect(screen.getByText('Access Duration')).toBeVisible();
  expect(screen.getByText('8 hours')).toBeVisible();

  // Assert the end summary emphasizes the end date and time.
  expect(screen.getByText(/Access will end on/)).toHaveTextContent(
    'Access will end on Oct 8, 2026 at 4:30 PM UTC'
  );
  expect(screen.getByText('Oct 8, 2026')).toBeVisible();
  expect(screen.getByText('4:30 PM')).toBeVisible();
});

test('future scheduled layout renders editable start and end controls', () => {
  // Setup a future scheduled window and its end-change callback.
  jest.useFakeTimers().setSystemTime(new Date('2026-10-19T20:00:00Z'));
  const start = new Date('2026-10-20T16:00:00Z');
  const end = new Date('2026-10-20T18:00:00Z');
  const onAccessEndChange = jest.fn();

  // Execute.
  render(
    <Validation>
      <ScheduledAccessTiming
        start={start}
        maxAllowedDuration={8 * 60 * 60 * 1000}
        durationOptions={[]}
        selectedAccessEnd={{ value: end.getTime(), label: '2 hours' }}
        onStartChange={jest.fn()}
        onAccessEndChange={onAccessEndChange}
      />
    </Validation>
  );

  // Assert both dates and times are editable and labelled by boundary.
  expect(screen.getByText('Access starts')).toBeVisible();
  expect(screen.getByText('Start time (UTC+0)')).toBeVisible();
  expect(screen.getByText('Access ends')).toBeVisible();
  expect(screen.getByText('End time (UTC+0)')).toBeVisible();
  expect(screen.getAllByText('Oct 20, 2026')).toHaveLength(2);
  expect(screen.getAllByRole('combobox')).toHaveLength(2);

  // Assert the selected times carry the time zone separately from the time.
  expect(screen.getByText('4:00 PM')).toBeVisible();
  expect(screen.getByText('6:00 PM')).toBeVisible();
  expect(screen.getAllByText('(UTC)')).toHaveLength(2);

  // Assert the duration is summarized on a single line.
  expect(screen.getByText(/Access duration:/)).toHaveTextContent(
    'Access duration: 2 hours'
  );

  // Execute opening the end-time options.
  const endTimeSelect = screen.getAllByRole('combobox')[1];
  fireEvent.focus(endTimeSelect);
  fireEvent.keyDown(endTimeSelect, { key: 'ArrowDown', keyCode: 40 });

  // Assert each end-time option is annotated with its duration from the start.
  expect(screen.getByText('2h 30m')).toBeVisible();
  expect(screen.getByText('7h 30m')).toBeVisible();

  // Execute an end-time change.
  const newEnd = new Date('2026-10-20T18:30:00Z');
  fireEvent.click(screen.getByText('6:30 PM'));

  // Assert the callback receives the new absolute end and duration.
  expect(onAccessEndChange).toHaveBeenCalledWith({
    value: newEnd.getTime(),
    label: getFormattedDurationTxt({ start, end: newEnd }),
  });
});
