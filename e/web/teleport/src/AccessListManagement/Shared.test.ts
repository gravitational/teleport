import { calculateMonthsDaysFromDuration, getFormattedDate } from './Shared';

describe('calculateMonthsDaysFromDuration', () => {
  // eslint-disable-next-line jest/require-hook
  [
    { duration: '0h0m0s', output: { months: 0, days: 0 } },
    { duration: '730h0m0s', output: { months: 1, days: 0 } },
    { duration: '1460h', output: { months: 2, days: 0 } },
    { duration: '2190h', output: { months: 3, days: 0 } },
    { duration: '2920h', output: { months: 4, days: 0 } },
    { duration: '3650h', output: { months: 5, days: 0 } },
    { duration: '4380h', output: { months: 6, days: 0 } },
    { duration: '', output: { months: 0, days: 0 } },
    { duration: '0h4320m', output: { months: 0, days: 3 } },
    { duration: '0h0m259200s', output: { months: 0, days: 3 } },
    { duration: '4380h4320m259200s', output: { months: 6, days: 6 } },
  ].forEach(tc => {
    test(`duration: ${tc.duration}`, () => {
      const obj = calculateMonthsDaysFromDuration(tc.duration);
      expect(obj).toStrictEqual(tc.output);
    });
  });
});

test('getFormattedDate', async () => {
  expect(getFormattedDate(null)).toBe('');
  expect(getFormattedDate(new Date('0001-01-01T00:00:00Z'))).toBe('');
  expect(getFormattedDate(undefined)).toBe('');
  expect(getFormattedDate(new Date(null))).toBe('');
  expect(getFormattedDate(new Date('2023-08-24T17:48:15.78579Z'))).toBe(
    '08/24/2023'
  );
});
