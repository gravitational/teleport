import React, { useEffect, useState } from 'react';
import { ButtonPrimary, Indicator, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import * as service from 'teleport/services/userPreferences';

import {
  LocalStorageSurvey,
  storageService,
} from 'teleport/services/storageService';

import useAttempt from 'shared/hooks/useAttemptNext';

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import {
  MarketingParamData,
  SetSurveyResultsRequest,
  SurveyCompanyResponse,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import { surveyService } from 'e-teleport/services/survey';

import { getMarketingResources } from 'e-teleport/Welcome/Questionnaire/getMarketingResources';

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
  onboard,
  username = '',
  onSubmit,
}: QuestionnaireProps): React.ReactElement => {
  const { attempt, run } = useAttempt('processing');

  const [formFields, setFormFields] = useState<QuestionnaireFormFields>({
    companyName: '',
    employeeCount: undefined,
    team: undefined,
    teamName: '',
    role: undefined,
    resources: [],
  });

  // we query Sales Center for Company questions to determine if any user on this cluster has answered them.
  // If true, we only show a partial survey.
  // If false, we show the entire survey.
  const [fullSurvey, setFullSurvey] = useState<boolean>(true);
  const [marketingPref, setMarketingPref] = useState<MarketingParamData>();

  useEffect(() => {
    async function getSurveyResults() {
      const resp: SurveyCompanyResponse =
        await surveyService.getSurveyCompanyResults();
      const marketingResources = await getMarketingResources(
        resp.marketingParams
      );

      setFullSurvey(resp.companyName == '' && resp.employeeCount == '');
      setMarketingPref(resp.marketingParams);
      setFormFields(previous => ({
        ...previous,
        // preselect resources which match marketing preferences
        resources: marketingResources,
      }));
    }

    // We're not leveraging any errors returned from getSurveyResults,
    // if the call fails we display the whole survey
    run(() => getSurveyResults());
  }, [run]);

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
    const clusterResources: Resource[] = formFields.resources.map(
      r => resourceMapping[ResourceOption[r]]
    );

    const request: SetSurveyResultsRequest = {
      companyName: formFields.companyName,
      employeeCount: formFields.employeeCount,
      resources: formFields.resources,
      role: formFields.role,
      team: formFields.team,
      username: username || '',
    };

    // When answering the survey, the user can either be in the onboarding or post-log-in modal flow.
    // If onboarding, we persist data to local storage for safe persistence after redirect.
    // If the user is post-log-in, we transmit the data immediately.
    if (onboard) {
      // set survey result and marketing params in localstorage
      const lsRequest: LocalStorageSurvey = {
        ...request,
        resources: formFields.resources,
        clusterResources: clusterResources,
        marketingParams: marketingPref,
      };
      storageService.setOnboardSurvey(lsRequest);

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
          marketingParams: marketingPref,
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

  return (
    <>
      {attempt.status === 'processing' ? (
        <Indicator />
      ) : (
        <>
          <Text typography="h2" mb={4}></Text>
          Tell us about yourself
          <Validation>
            {({ validator }) => (
              <>
                {fullSurvey && (
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
      )}
    </>
  );
};
