import { Option } from 'shared/components/Select';

import { FormDataField } from '../types';

export type FilterOption = Option & { invalid: boolean };

export type FormDataFilterField =
  | FormDataField.AppFilters
  | FormDataField.GroupFilters;
