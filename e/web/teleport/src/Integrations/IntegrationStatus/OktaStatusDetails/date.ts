import { formatDistanceStrict } from 'date-fns';

export function getDurationText(date: Date) {
  if (!date || date.getTime() <= 0) {
    return 'not recorded yet';
  }

  return formatDistanceStrict(new Date(date), new Date(), { addSuffix: true });
}
