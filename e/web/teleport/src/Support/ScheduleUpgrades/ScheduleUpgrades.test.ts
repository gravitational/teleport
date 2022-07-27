import { makeLabel } from './ScheduleUpgrades';

describe('makeLabel', () => {
  test.each`
    window     | expected
    ${'08:00'} | ${'08:00 (UTC)'}
    ${'16:00'} | ${'16:00 (UTC)'}
    ${'23:00'} | ${'23:00 (UTC)'}
  `('upgrade window $window produces $expected', ({ window, expected }) => {
    const label = makeLabel(window);
    expect(label).toEqual(expected);
  });
});
