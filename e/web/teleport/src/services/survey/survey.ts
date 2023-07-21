import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import { SurveyRequest } from './types';

export const surveyService = {
  submitSurvey(survey: SurveyRequest) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.surveyPath, {
      method: 'POST',
      body: JSON.stringify(survey),
    });
  },
};
