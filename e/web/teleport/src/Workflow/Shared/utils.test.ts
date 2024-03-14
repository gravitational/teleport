import { getStartDateTime } from './utils';

test('getStartDateTime', () => {
  expect(getStartDateTime(null)).toBeUndefined();

  expect(
    getStartDateTime({
      date: new Date('2022-12-20T19:14:07.763Z'),
      time: { value: { militaryHrs: 13, minutes: 45 }, label: '' },
    })
  ).toStrictEqual(new Date('2022-12-20T13:45:00.000Z'));
});
