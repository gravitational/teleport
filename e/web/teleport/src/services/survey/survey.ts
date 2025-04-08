import cfg from 'e-teleport/config';
import { SetSurveyResultsRequest } from 'e-teleport/services/cloud/v1/tenants_pb';
import api from 'teleport/services/api';

export const surveyService = {
  submitSurvey(survey: Omit<SetSurveyResultsRequest, 'username'>) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.surveyPath, {
      method: 'POST',
      body: JSON.stringify(survey),
    });
  },
};
