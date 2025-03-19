import { getUnixTime } from 'date-fns';

import { UsageHistoryItem } from 'e-teleport/services/cloud/v1/tenants_pb';

export const usageHistory: UsageHistoryItem[] = [
  {
    mau: 102,
    tpr: 1002,
    mwi: 52,
    igmau: 42,
    cycleStart: getUnixTime(new Date('2023/03/15')),
    cycleStartFormatted: 'Mar 15, 2023',
    cycleEnd: getUnixTime(new Date('2023/04/14')),
    cycleEndFormatted: 'Apr 14, 2023',
  },
  {
    mau: 101,
    tpr: 1001,
    mwi: 51,
    igmau: 41,
    cycleStart: getUnixTime(new Date('2023/02/15')),
    cycleStartFormatted: 'Feb 15, 2023',
    cycleEnd: getUnixTime(new Date('2023/03/14')),
    cycleEndFormatted: 'Mar 14, 2023',
  },
  {
    mau: 100,
    tpr: 1000,
    mwi: 20,
    igmau: 10,
    cycleStart: getUnixTime(new Date('2023/01/15')),
    cycleStartFormatted: 'Jan 15, 2023',
    cycleEnd: getUnixTime(new Date('2023/02/14')),
    cycleEndFormatted: 'Feb 14, 2023',
  },
];
