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
