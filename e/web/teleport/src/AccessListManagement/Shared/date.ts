import { format } from 'date-fns';

export const dateFormat = 'MM/dd/yyyy';

export function getFormattedDate(d: Date) {
  if (!d || isNaN(d.getTime())) {
    return '';
  }

  // The zero value for golang Date comes back as "0001-01-01T00:00:00Z"
  // which is January 1, year 1.
  // JS zero date is January 1, 1970  which is "greater" than
  // golang's zero value. So it's safe to assume that backend Dates
  // that are less than JS's zero value means the date was not set.
  const zeroDate = new Date(0);
  const thisDate = new Date(d);
  if (thisDate <= zeroDate) {
    return '';
  }

  return format(thisDate, dateFormat);
}
