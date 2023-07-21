export type QuestionnaireProps = {
  // full indicates if a full survey should be presented,
  // false indicates that a partial survey is shown (some questions are skipped)
  full: boolean;
  username: string;
  // optional callback to handle parent interaction
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
