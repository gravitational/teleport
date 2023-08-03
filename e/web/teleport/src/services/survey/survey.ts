import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import { CompanySurveyDTO, SurveyDTO } from './types';

export const surveyService = {
  submitSurvey(survey: SurveyDTO) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.surveyPath, {
      method: 'POST',
      body: JSON.stringify(survey),
    });
  },

  getSurveyCompanyResults(): Promise<CompanySurveyDTO> {
    return api.get(cfg.api.surveyCompanyPath);
  },
};
