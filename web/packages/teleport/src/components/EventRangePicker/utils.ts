import moment from 'moment';

export function getRangeOptions(): EventRange[] {
  return [
    {
      name: 'Today',
      from: moment(new Date())
        .startOf('day')
        .toDate(),
      to: moment(new Date())
        .endOf('day')
        .toDate(),
    },
    {
      name: '7 days',
      from: moment()
        .subtract(6, 'day')
        .startOf('day')
        .toDate(),
      to: moment(new Date())
        .endOf('day')
        .toDate(),
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
