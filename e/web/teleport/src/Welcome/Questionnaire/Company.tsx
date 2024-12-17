import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { requiredField } from 'shared/components/Validation/rules';

import { EmployeeSelectOption, EmployeeSelectOptions } from './constants';
import { CompanyProps } from './types';

export const Company = ({
  updateFields,
  companyName,
  numberOfEmployees,
}: CompanyProps) => (
  <>
    <FieldInput
      label="Company Name"
      rule={requiredField('Company Name is required')}
      type="text"
      value={companyName}
      placeholder="ex. GitHub"
      onChange={e => {
        updateFields({ companyName: e.target.value });
      }}
    />
    <FieldSelect<EmployeeSelectOption>
      label="Number of Employees"
      rule={requiredField('Number of Employees is required')}
      placeholder="Select Company Size"
      onChange={(e: EmployeeSelectOption) =>
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
