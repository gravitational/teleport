/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { ButtonPrimary, Indicator, Text, Flex, ButtonSecondary } from 'design';
import Validation, { Validator } from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { Role } from './Role';
import { Resources } from './Resources';
import { Company } from './Company';
import { QuestionnaireFormFields } from './types';

export type QuestionnaireComponentProps = {
  formFields: QuestionnaireFormFields;
  updateForm(f: QuestionnaireFormFields): void;
  attempt?: Attempt;
  submitForm(v: Validator, skip?: boolean): Promise<void>;
  wantFullSurvey?: boolean;
  canSkip?: boolean;
};

export function QuestionnaireComponent({
  attempt,
  submitForm,
  wantFullSurvey = true,
  formFields,
  updateForm,
  canSkip = false,
}: QuestionnaireComponentProps) {
  if (attempt?.status === 'processing') {
    return <Indicator />;
  }
  return (
    <>
      <Text typography="h2" mb={4}></Text>
      Tell us about yourself
      <Validation>
        {({ validator }) => (
          <>
            {wantFullSurvey && (
              <Company
                companyName={formFields.companyName}
                numberOfEmployees={formFields.employeeCount}
                updateFields={updateForm}
              />
            )}
            <Role
              role={formFields.role}
              team={formFields.team}
              teamName={formFields.teamName}
              updateFields={updateForm}
            />
            <Resources
              checked={formFields.resources}
              updateFields={updateForm}
            />
            <Flex gap={3} mt={3}>
              {canSkip ? (
                <>
                  <ButtonPrimary
                    width="50%"
                    size="large"
                    onClick={() => submitForm(validator)}
                  >
                    Submit
                  </ButtonPrimary>
                  <ButtonSecondary
                    width="50%"
                    size="large"
                    onClick={() => submitForm(validator, true)}
                  >
                    Skip
                  </ButtonSecondary>
                </>
              ) : (
                <ButtonPrimary
                  width="100%"
                  size="large"
                  onClick={() => submitForm(validator)}
                >
                  Submit
                </ButtonPrimary>
              )}
            </Flex>
          </>
        )}
      </Validation>
    </>
  );
}
