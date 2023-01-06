import { displayDate, displayDateTime } from './loc';

const testDate = new Date('2022-01-28T16:00:44.309Z');

test('displayDate', () => {
  const output = displayDate(testDate);

  expect(output).toBe('2022-01-28');
});

test('displayDateTime', () => {
  const output = displayDateTime(testDate);

  expect(output).toBe('2022-01-28 16:00:44');
});
