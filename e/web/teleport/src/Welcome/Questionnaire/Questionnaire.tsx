import React, { useState } from 'react';
import { ButtonPrimary, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import { ClusterResource } from 'teleport/services/userPreferences/types';
import * as service from 'teleport/services/userPreferences';

import localStorage, {
  LocalStorageSurvey,
  SurveyRequest,
} from 'teleport/services/localStorage';

import { surveyService } from 'e-teleport/services/survey';

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
  onboard,
  username = '',
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

  const submitForm = async (validator: Validator) => {
    if (!validator.validate()) {
      return;
    }

    // maps the string enum used for UI display with proto int
    const clusterResources: ClusterResource[] = formFields.resources.map(
      r => resourceMapping[ResourceOption[r]]
    );

    const request: SurveyRequest = {
      companyName: formFields.companyName,
      employeeCount: formFields.employeeCount,
      resources: formFields.resources,
      role: formFields.role,
      team: formFields.team,
    };

    if (onboard) {
      // set survey result in localstorage, because we do not have a bearer-token this
      // early in onboarding (will be sent when onboarding completes)
      const lsRequest: LocalStorageSurvey = {
        ...request,
        clusterResources: clusterResources,
      };
      localStorage.setOnboardSurvey(lsRequest);

      if (username) {
        // submit a pre-user posthog event
        userEventService.capturePreUserEvent({
          event: CaptureEvent.OnboardQuestionnaireSubmitEvent,
          username: username,
        });
      }
    } else {
      // submit answers to BE for storage in Sales Center
      surveyService.submitSurvey(request);

      // set resources on new user preferences cluster state
      await service.updateUserPreferences({
        onboard: {
          preferredResources: clusterResources,
        },
      });

      // submit a posthog event
      userEventService.captureUserEvent({
        event: CaptureEvent.OnboardQuestionnaireSubmitEvent,
      });
    }

    // callback to continue flow
    if (onSubmit) {
      onSubmit();
    }
  };

  // todo (michellescripts) only display <Company .../> if the survey is unanswered for the account
  return (
    <>
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
    </>
  );
};
