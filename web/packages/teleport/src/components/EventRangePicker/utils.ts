import { startOfDay, endOfDay, subDays } from 'date-fns';

export function getRangeOptions(): EventRange[] {
  return [
    {
      name: 'Today',
      from: startOfDay(new Date()),
      to: endOfDay(new Date()),
    },
    {
      name: '7 days',
      from: startOfDay(subDays(new Date(), 6)),
      to: endOfDay(new Date()),
    },
    {
      name: 'Custom Range...',
      isCustom: true,
      from: new Date(),
      to: new Date(),
    },
  ];
}

export type EventRange = {
  from: Date;
  to: Date;
  isCustom?: boolean;
  name?: string;
};
