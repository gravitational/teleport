import { pluralize } from 'teleport/lib/util';

export function getDurationText(hrs: number, mins: number, secs: number) {
  if (!hrs && !mins) {
    return `${secs} secs`;
  }

  const hrText = pluralize(hrs, 'hr');
  const minText = pluralize(mins, 'min');

  if (!hrs) {
    return `${mins} ${minText}`;
  }

  if (hrs && !mins) {
    return `${hrs} ${hrText}`;
  }

  return `${hrs} ${hrText} and ${mins} ${minText}`;
}
