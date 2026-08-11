import { Filters } from '../types';

export const emptyFilter: Filters = {
  id: [],
  nameRegex: [],
  excludeId: [],
  excludeNameRegex: [],
};

type filter = {
  name: keyof Filters;
  label: string;
  placeholder: string;
};

/**
 * filterCollection defines the supported filter modes for the Entra ID plugin.
 */
export const filterCollection: filter[] = [
  {
    name: 'id',
    label: 'Include Groups Matching the Specified Group IDs',
    placeholder: 'Type a group ID and press enter',
  },
  {
    name: 'nameRegex',
    label:
      'Include Groups Matching the Specified Group Name(s) - Regex and Glob Supported',
    placeholder:
      'Type a group name, regex or glob matching group name(s) and press enter',
  },
  {
    name: 'excludeId',
    label: 'Exclude Groups Matching the Specified Group IDs',
    placeholder: 'Type a group ID and press enter',
  },
  {
    name: 'excludeNameRegex',
    label:
      'Exclude Groups Matching the Specified Group Name(s) - Regex and Glob Supported',
    placeholder:
      'Type a group name, regex or glob matching group name(s) and press enter',
  },
];
