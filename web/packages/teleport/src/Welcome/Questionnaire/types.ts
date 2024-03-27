/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
  ONE = '1 - 9',
  TWO = '10-19',
  THREE = '20-199',
  FOUR = '200-499',
  FIVE = '500-999',
  SIX = '1000-4999',
  SEVEN = '5,000-9,999',
  EIGHT = '10,000-19,999',
  NINE = '20,000+',
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

export type ResourceKey = keyof typeof ResourceOption;

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
