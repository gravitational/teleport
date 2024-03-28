import { Option } from 'shared/components/Select';

export type TimeOption = Option<Date>;

export type CreateRequest = {
  reason?: string;
  start?: Date;
  suggestedReviewers?: string[];
  maxDuration?: Date;
  requestTTL?: Date;
  dryRun?: boolean;
};
