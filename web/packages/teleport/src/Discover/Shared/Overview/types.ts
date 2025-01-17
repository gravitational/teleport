import { ReactElement } from 'react';

export interface Prerequisite {
  content: string;
  details?: string[];
}

export interface Overview {
  OverviewContent: () => ReactElement;
  PrerequisiteContent: () => ReactElement;
}
