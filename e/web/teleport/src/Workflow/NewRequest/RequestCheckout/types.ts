import { Option } from 'shared/components/Select';

export type ReviewerOption = Option & {
  isDisabled?: boolean;
  isSelected?: boolean;
};
