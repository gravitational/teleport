import { makeLabel } from './ScheduleUpgrades';

describe('makeLabel', () => {
  test.each`
    window | expected
    ${8}   | ${'08:00 (UTC)'}
    ${16}  | ${'16:00 (UTC)'}
    ${23}  | ${'23:00 (UTC)'}
  `('upgrade window $window produces $expected', ({ window, expected }) => {
    const label = makeLabel(window);
    expect(label).toEqual(expected);
  });
});
