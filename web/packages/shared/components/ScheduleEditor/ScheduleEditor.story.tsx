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

import { useState } from 'react';

import { ButtonPrimary } from 'design/Button';
import Validation from 'shared/components/Validation';

import { ScheduleEditor } from './ScheduleEditor';
import { newSchedule, Schedule } from './types';

export default {
  title: 'Teleport/ScheduleEditor',
};

export const ScheduleEditorBasic = () => {
  const [schedule, setSchedule] = useState<Schedule>(newSchedule());
  return (
    <Validation>
      {({ validator }) => (
        <>
          <ScheduleEditor schedule={schedule} setSchedule={setSchedule} />
          <ButtonPrimary onClick={() => validator.validate()}>
            Validate
          </ButtonPrimary>
        </>
      )}
    </Validation>
  );
};
