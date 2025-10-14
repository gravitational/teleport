/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { render, screen, userEvent } from 'design/utils/testing';
import Validation from 'shared/components/Validation';

import { ScheduleEditor } from './ScheduleEditor';
import { newSchedule } from './types';

test('Toggle Monday', async () => {
  const schedule = newSchedule();
  const setSchedule = jest.fn();

  render(
    <Validation>
      <ScheduleEditor schedule={schedule} setSchedule={setSchedule} />
    </Validation>
  );

  const user = userEvent.setup();
  const mondayButton = screen.getByRole('button', { name: /M/i });
  await user.click(mondayButton);

  expect(setSchedule).toHaveBeenCalledWith({
    ...schedule,
    shifts: {
      ...schedule.shifts,
      Monday: {
        startTime: { label: '12:00AM', value: '00:00' },
        endTime: { label: '12:00AM', value: '00:00' },
      },
    },
  });
});

test('Set custom Monday start time', async () => {
  const schedule = newSchedule();
  const setSchedule = jest.fn();

  schedule.shifts.Monday = {
    startTime: { label: '12:00AM', value: '00:00' },
    endTime: { label: '12:00AM', value: '00:00' },
  };

  render(
    <Validation>
      <ScheduleEditor schedule={schedule} setSchedule={setSchedule} />
    </Validation>
  );

  const user = userEvent.setup();
  expect(screen.getAllByText('12:00AM')).toHaveLength(2);

  const startSelector = screen.getAllByRole('combobox')[1];
  await user.click(startSelector);
  await user.type(startSelector, '9:10AM');
  const startOption = await screen.findByText('Create "9:10AM"');
  await user.click(startOption);

  expect(setSchedule).toHaveBeenCalledWith({
    ...schedule,
    shifts: {
      ...schedule.shifts,
      Monday: {
        startTime: { label: '9:10AM', value: '09:10' },
        endTime: { label: '12:00AM', value: '00:00' },
      },
    },
  });
});
