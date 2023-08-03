// Note: QuestionnaireProps is duplicated in OSS (teleport/Welcome/NewCredentials)
export type QuestionnaireProps = {
  // Onboard indicates if the questionnaire is being shown during onboarding (true) or
  // after login (false). This impacts the submission of the form.
  onboard: boolean;
  // Username is optional; it is only required during the onboarding flow to submit a posthog event.
  // After login, we use the auth endpoint to set the user.
  username?: string;
  // onSubmit is an optional callback to handle parent interaction.
  onSubmit?: () => void;
};

export type QuestionProps = {
  updateFields: (fields: Partial<QuestionnaireFormFields>) => void;
};

export type CompanyProps = QuestionProps & {
  companyName: string;
  numberOfEmployees: EmployeeOption;
};

export type RoleProps = QuestionProps & {
  role: TitleOption;
  team: TeamOption;
  teamName: string;
};

export type ResourceType = {
  label: ResourceOption;
  image: string;
};

export type ResourcesProps = QuestionProps & {
  checked: ResourceOption[];
};

export enum EmployeeOption {
  ONE = '0-19',
  TWO = '20-199',
  THREE = '200-499',
  FOUR = '500-999',
  FIVE = '1000-4999',
  SIX = '5000+',
}

export enum TeamOption {
  SOFTWARE_ENGINEERING = 'Software Engineering',
  DEVOPS_ENGINEERING = 'DevOps Engineering',
  IT = 'IT',
  SUPPORT = 'Support',
  FINANCE = 'Finance',
  LEGAL = 'Legal',
  OTHER = 'Other (free-form field)',
}

export enum TitleOption {
  INDIVIDUAL_CONTRIBUTOR = 'Individual contributor',
  MANAGER = 'Manager',
  DIRECTOR = 'Director',
  VP = 'VP',
  C_SUITE_OWNER = 'C-Suite/Owner',
}

export enum ResourceOption {
  RESOURCE_WEB_APPLICATIONS = 'Web Applications',
  RESOURCE_WINDOWS_DESKTOPS = 'Windows Desktops',
  RESOURCE_SERVER_SSH = 'Server/SSH',
  RESOURCE_DATABASES = 'Databases',
  RESOURCE_KUBERNETES = 'Kubernetes',
}

export type QuestionnaireFormFields = {
  companyName: string;
  employeeCount: EmployeeOption;
  role: TitleOption;
  team: TeamOption;
  resources: ResourceOption[];
  teamName: string;
};
