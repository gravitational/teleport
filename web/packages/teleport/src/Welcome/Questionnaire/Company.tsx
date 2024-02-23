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
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { requiredField } from 'shared/components/Validation/rules';

import { EmployeeSelectOptions } from './constants';
import { CompanyProps, EmployeeOption } from './types';

export const Company = ({
  updateFields,
  companyName,
  numberOfEmployees,
}: CompanyProps) => (
  <>
    <FieldInput
      label="Company Name"
      rule={requiredField('Company Name is required')}
      id="company-name"
      type="text"
      value={companyName}
      placeholder="ex. GitHub"
      onChange={e => {
        updateFields({ companyName: e.target.value });
      }}
    />
    <FieldSelect
      label="Number of Employees"
      rule={requiredField('Number of Employees is required')}
      placeholder="Select Company Size"
      onChange={(e: Option<EmployeeOption>) =>
        updateFields({ employeeCount: e.value })
      }
      value={
        numberOfEmployees
          ? {
              label: numberOfEmployees,
              value: numberOfEmployees,
            }
          : null
      }
      options={EmployeeSelectOptions}
    />
  </>
);
