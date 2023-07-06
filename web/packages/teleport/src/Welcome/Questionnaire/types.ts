/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
  '0-19',
  '20-199',
  '200-499',
  '500-999',
  '1000-4999',
  '5000+',
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

export enum ProtoResource {
  RESOURCE_UNSPECIFIED = 0,
  RESOURCE_WINDOWS_DESKTOPS = 1,
  RESOURCE_SERVER_SSH = 2,
  RESOURCE_DATABASES = 3,
  RESOURCE_KUBERNETES = 4,
  RESOURCE_WEB_APPLICATIONS = 5,
}

export enum ResourceOption {
  RESOURCE_WINDOWS_DESKTOPS = 'Windows Desktops',
  RESOURCE_SERVER_SSH = 'Server/SSH',
  RESOURCE_DATABASES = 'Databases',
  RESOURCE_KUBERNETES = 'Kubernetes',
  RESOURCE_WEB_APPLICATIONS = 'Web Applications',
}

export type QuestionnaireFormFields = {
  companyName: string;
  employeeCount: EmployeeOption;
  role: TitleOption;
  team: TeamOption;
  resources: ResourceOption[];
  teamName: string;
};
