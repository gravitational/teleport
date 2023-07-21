import React, { useState } from 'react';
import { ButtonPrimary, Card, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import { ClusterResource } from 'teleport/services/userPreferences/types';

import localStorage, {
  LocalStorageSurvey,
} from 'teleport/services/localStorage';

import {
  QuestionnaireFormFields,
  QuestionnaireProps,
  ResourceOption,
} from './types';
import { Company } from './Company';
import { Role } from './Role';
import { Resources } from './Resources';
import { resourceMapping } from './constants';

export const Questionnaire = ({
  full,
  username,
  onSubmit,
}: QuestionnaireProps): React.ReactElement => {
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

  const submitForm = (validator: Validator) => {
    if (!validator.validate()) {
      return;
    }

    // maps the string enum used for UI display with proto int
    const clusterResources: ClusterResource[] = formFields.resources.map(
      r => resourceMapping[ResourceOption[r]]
    );

    const request: LocalStorageSurvey = {
      companyName: formFields.companyName,
      employeeCount: formFields.employeeCount,
      resources: formFields.resources,
      clusterResources: clusterResources,
      role: formFields.role,
      team: formFields.team,
    };

    // set survey result in localstorage, because we do not have a bearer-token this
    // early in onboarding (will be sent when onboarding completes)
    localStorage.setOnboardSurvey(request);

    // submit a posthog event
    userEventService.capturePreUserEvent({
      event: CaptureEvent.PreUserOnboardQuestionnaireSubmitEvent,
      username: username,
    });

    // callback to continue flow
    if (onSubmit) {
      onSubmit();
    }
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
            {full && (
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
