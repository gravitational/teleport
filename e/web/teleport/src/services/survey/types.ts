export type CompanySurveyDTO = {
  companyName: string;
  employeeCount: string;
};

export type SurveyDTO = CompanySurveyDTO & {
  resources: Array<string>;
  role: string;
  team: string;
};
