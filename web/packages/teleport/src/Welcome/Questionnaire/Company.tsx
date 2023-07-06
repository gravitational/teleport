/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
      placeholder="Select Team Size"
      onChange={(e: Option<EmployeeOption>) =>
        updateFields({ employeeCount: e.value })
      }
      value={
        numberOfEmployees
          ? { label: numberOfEmployees, value: numberOfEmployees }
          : null
      }
      options={EmployeeSelectOptions}
    />
  </>
);
