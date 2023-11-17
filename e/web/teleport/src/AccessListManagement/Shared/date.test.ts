import { getFormattedDate } from './date';

test('getFormattedDate', async () => {
  expect(getFormattedDate(null)).toBe('');
  expect(getFormattedDate(new Date('0001-01-01T00:00:00Z'))).toBe('');
  expect(getFormattedDate(undefined)).toBe('');
  expect(getFormattedDate(new Date(null))).toBe('');
  expect(getFormattedDate(new Date('2023-08-24T17:48:15.78579Z'))).toBe(
    '08/24/2023'
  );
});
