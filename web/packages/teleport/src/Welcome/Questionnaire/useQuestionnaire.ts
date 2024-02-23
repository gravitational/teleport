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

import { useState } from 'react';

import { QuestionnaireFormFields } from 'teleport/Welcome/Questionnaire/types';

export function useQuestionnaire() {
  const [formFields, setFormFields] = useState<QuestionnaireFormFields>({
    companyName: '',
    employeeCount: undefined,
    team: undefined,
    teamName: '',
    role: undefined,
    resources: [],
  });

  const updateForm = (fields: Partial<QuestionnaireFormFields>) => {
    setFormFields({
      role: fields.role ?? formFields.role,
      team: fields.team ?? formFields.team,
      teamName: fields.teamName ?? formFields.teamName,
      resources: fields.resources ?? formFields.resources,
      companyName: fields.companyName ?? formFields.companyName,
      employeeCount: fields.employeeCount ?? formFields.employeeCount,
    });
  };

  return {
    formFields,
    setFormFields,
    updateForm,
  };
}
