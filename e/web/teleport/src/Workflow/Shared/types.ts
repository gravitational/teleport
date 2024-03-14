import { Option } from 'shared/components/Select';

export type Time = {
  minutes: number;
  /**
   * militaryHrs is 24 hour time eg: 16 (as 16:00) is 4 (4PM)
   */
  militaryHrs: number;
};

export type TimeOption = Option<Time>;

export type Start = {
  date: Date;
  time: TimeOption;
};

export type CreateRequest = {
  reason?: string;
  start?: Date;
  suggestedReviewers?: string[];
  maxDuration?: Date;
  requestTTL?: Date;
  dryRun?: boolean;
};
