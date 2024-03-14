import { addDays } from 'date-fns';
import { Option } from 'shared/components/Select';

import { AccessRequest } from 'e-teleport/services/workflow';

import { TimeOption } from '../Shared/types';

import {
  getDurationOptionsFromStartTime,
  presetDays,
  presetHours,
} from './utils';

test('duration difference is less than an hour returns only the max duration', () => {
  const created = new Date('2024-02-16T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-16T03:45:00.156944Z');
  const selectedDate = new Date(created);

  const startTime: TimeOption = {
    value: { minutes: 0, militaryHrs: 3 },
    label: '',
  };

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(
    selectedDate,
    startTime,
    mockAccessRequest
  );

  // Only one option, the max duration.
  expect(opts).toHaveLength(1);
  expect(opts[0].value).toBe(maxDuration.getTime());
  expect(opts[0].label).toBe('45 minutes (Max Duration)');
});

test('duration difference is 1hr 30min, returns an hour option and the max duration', () => {
  const created = new Date('2024-02-16T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-16T04:30:00.156944Z');
  const selectedDate = new Date(created);

  const startTime: TimeOption = {
    value: { minutes: 0, militaryHrs: 3 },
    label: '',
  };

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(
    selectedDate,
    startTime,
    mockAccessRequest
  );

  expect(opts).toHaveLength(2);

  // one preset hour
  expect(opts[0].value).toBe(
    new Date(selectedDate).setHours(
      startTime.value.militaryHrs + presetHours[0],
      0,
      0,
      0
    )
  );
  expect(opts[0].label).toBe('1 hour');

  // max duration
  expect(opts[1].value).toBe(maxDuration.getTime());
  expect(opts[1].label).toBe('1 hour 30 minutes (Max Duration)');
});

test('defining all preset hours', () => {
  expect(presetHours).toHaveLength(8);

  const created = new Date('2024-02-16T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-16T21:00:00.156944Z');
  const selectedDate = new Date(created);

  const startTime: TimeOption = {
    value: { minutes: 0, militaryHrs: 3 },
    label: '',
  };

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(
    selectedDate,
    startTime,
    mockAccessRequest
  );

  expect(opts).toHaveLength(presetHours.length);
  testPresetHours(
    opts,
    selectedDate,
    startTime,
    true /* has max duration among preset hours */
  );
});

test('defining all preset days + preset hours + maxest duration', () => {
  expect(presetHours).toHaveLength(8);
  expect(presetDays).toHaveLength(14);

  const created = new Date('2024-02-11T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-25T03:30:00.156944Z');
  const selectedDate = new Date(created);

  const startTime: TimeOption = {
    value: { minutes: 0, militaryHrs: 3 },
    label: '',
  };

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(
    selectedDate,
    startTime,
    mockAccessRequest
  );

  expect(opts).toHaveLength(presetHours.length + presetDays.length + 1);
  testPresetHours(
    opts,
    selectedDate,
    startTime,
    false /* no max duration among preset hours */
  );

  const startDateTime = new Date(created);
  startDateTime.setHours(startTime.value.militaryHrs, 0, 0, 0);
  for (let i = 0; i < presetDays.length; i += 1) {
    const optionIndex = i + presetHours.length;
    expect(opts[optionIndex].value).toBe(
      addDays(startDateTime, presetDays[i]).getTime()
    );
    const dayTxt = i ? 'days' : 'day';
    expect(opts[optionIndex].label).toBe(`${presetDays[i]} ${dayTxt}`);
  }

  // Test maxest duration.
  expect(opts[opts.length - 1].value).toBe(maxDuration.getTime());
  expect(opts[opts.length - 1].label).toBe('14 days 30 minutes (Max Duration)');
});

function testPresetHours(
  opts: Option<number>[],
  selectedDate: Date,
  startTime: TimeOption,
  hasMaxDuration: boolean
) {
  // one preset hour
  for (let i = 0; i < presetHours.length; i += 1) {
    expect(opts[i].value).toBe(
      new Date(selectedDate).setHours(
        startTime.value.militaryHrs + presetHours[i],
        0, // min
        0, // sec
        0 // ms
      )
    );
    const hourTxt = i ? 'hours' : 'hour';
    const maxDurationTxt =
      hasMaxDuration && i == presetHours.length - 1 ? ' (Max Duration)' : '';
    expect(opts[i].label).toBe(`${presetHours[i]} ${hourTxt}${maxDurationTxt}`);
  }
}

const mockAccessRequest: AccessRequest = {
  id: '31a711f6-f53a-4d61-baae-2c3c8d9a3fd9',
  state: 'PENDING',
  resolveReason: '',
  requestReason: '',
  user: 'lisa',
  roles: ['@teleport-access-approver'],
  created: new Date('2024-02-16T03:00:08.156944Z'),
  createdDuration: '',
  expires: new Date('2024-02-19T03:00:08.157365Z'),
  expiresDuration: '',
  maxDuration: new Date('2024-02-19T03:00:08.157365Z'),
  maxDurationText: '',
  requestTTL: new Date('2024-02-16T04:00:08.157365Z'),
  requestTTLDuration: '',
  sessionTTL: new Date('2024-02-16T12:11:46.99997Z'),
  sessionTTLDuration: '',
  reviews: [],
  reviewers: [],
  thresholdNames: ['default'],
  resources: [],
  assumeStartTime: null,
};
