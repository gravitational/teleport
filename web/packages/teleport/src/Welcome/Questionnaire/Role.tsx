/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import { Option } from 'shared/components/Select';
import FieldSelect from 'shared/components/FieldSelect';
import { requiredField } from 'shared/components/Validation/rules';
import FieldInput from 'shared/components/FieldInput';

import { RoleProps, TeamOption, TitleOption } from './types';
import { teamSelectOptions, titleSelectOptions } from './constants';

export const Role = ({ team, teamName, role, updateFields }: RoleProps) => (
  <>
    <FieldSelect
      label="Which Team are you on?"
      rule={requiredField('Team is required')}
      placeholder="Select Team"
      onChange={(e: Option<TeamOption>) => updateFields({ team: e.value })}
      options={teamSelectOptions}
      value={
        team
          ? {
              value: team,
              label: TeamOption[team],
            }
          : null
      }
    />
    {TeamOption[team] === TeamOption.OTHER && (
      <FieldInput
        id="team-name"
        type="text"
        label="Team Name"
        rule={requiredField('Team Name is required')}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          updateFields({ teamName: e.target.value })
        }
        value={teamName}
      />
    )}
    <FieldSelect
      label="Job Title"
      rule={requiredField('Job Title is required')}
      placeholder="Select Job Title"
      onChange={(e: Option<TitleOption>) => updateFields({ role: e.value })}
      options={titleSelectOptions}
      value={
        role
          ? {
              value: role,
              label: TitleOption[role],
            }
          : null
      }
    />
  </>
);
