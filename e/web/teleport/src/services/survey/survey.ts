import api from 'teleport/services/api';

import {
  SetSurveyResultsRequest,
  SurveyCompanyResponse,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import cfg from 'e-teleport/config';

export const surveyService = {
  submitSurvey(survey: Omit<SetSurveyResultsRequest.AsObject, 'username'>) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.surveyPath, {
      method: 'POST',
      body: JSON.stringify(survey),
    });
  },

  getSurveyCompanyResults(): Promise<SurveyCompanyResponse.AsObject> {
    return api.get(cfg.api.surveyCompanyPath);
  },
};
