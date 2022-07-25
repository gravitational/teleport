import { displayDate, displayDateTime } from 'shared/services/loc';

import { MatchCallback } from './match';

export function dateMatcher<T>(
  datePropNames: (keyof T & string)[]
): MatchCallback<T> {
  return (targetValue, searchValue, propName) => {
    if (datePropNames.includes(propName)) {
      return displayDate(targetValue).toLocaleUpperCase().includes(searchValue);
    }
  };
}

export function dateTimeMatcher<T>(
  dateTimePropNames: (keyof T & string)[]
): MatchCallback<T> {
  return (targetValue, searchValue, propName) => {
    if (dateTimePropNames.includes(propName)) {
      return displayDateTime(targetValue)
        .toLocaleUpperCase()
        .includes(searchValue);
    }
  };
}
