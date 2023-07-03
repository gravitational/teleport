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

import React, { useState } from 'react';
import { ButtonPrimary, Card, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

import { QuestionnaireFormFields, QuestionnaireProps } from './types';
import { Company } from './Company';
import { Role } from './Role';
import { Resources } from './Resources';
import { supportedResources } from './constants';

export const Questionnaire = ({ username }: QuestionnaireProps) => {
  const [formFields, setFormFields] = useState<QuestionnaireFormFields>({
    companyName: '',
    employeeCount: undefined,
    team: undefined,
    role: undefined,
    resources: [],
  });

  const updateForm = (fields: Partial<QuestionnaireFormFields>) => {
    setFormFields({
      role: fields.role ?? formFields.role,
      team: fields.team ?? formFields.team,
      resources: fields.resources ?? formFields.resources,
      companyName: fields.companyName ?? formFields.companyName,
      employeeCount: fields.employeeCount ?? formFields.employeeCount,
    });
  };

  const submitForm = (validator: Validator) => {
    if (!validator.validate()) {
      return;
    }

    // submit Posthog event
    userEventService.capturePreUserEvent({
      event: CaptureEvent.PreUserOnboardQuestionnaireSubmitEvent,
      username: username,
    });

    // todo (michellescripts) submit all Qs to Sales Center
    // todo (michellescripts) set resource Q on user state
  };

  // todo (michellescripts) only display <Company .../> if the survey is unanswered for the account
  return (
    <Card mx="auto" maxWidth="600px" p="4">
      <Text typography="h2" mb={4}>
        Tell us about yourself
      </Text>
      <Validation>
        {({ validator }) => (
          <>
            <Company
              companyName={formFields.companyName}
              numberOfEmployees={formFields.employeeCount}
              updateFields={updateForm}
            />
            <Role
              role={formFields.role}
              team={formFields.team}
              updateFields={updateForm}
            />
            <Resources
              resources={supportedResources}
              checked={formFields.resources}
              updateFields={updateForm}
            />

            <ButtonPrimary
              mt={3}
              width="100%"
              size="large"
              onClick={() => submitForm(validator)}
            >
              Submit
            </ButtonPrimary>
          </>
        )}
      </Validation>
    </Card>
  );
};
