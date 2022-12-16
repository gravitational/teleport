export enum DatabaseLocation {
  AWS,
  SelfHosted,
}

export enum DatabaseEngine {
  PostgreSQL,
  MySQL,
  SQLServer,
  RedShift,
  Mongo,
  Redis,
}

export interface Database {
  location: DatabaseLocation;
  engine: DatabaseEngine;
  name: string;
  popular?: boolean;
}

export const DATABASES: Database[] = [
  {
    location: DatabaseLocation.AWS,
    engine: DatabaseEngine.PostgreSQL,
    name: 'AWS RDS PostgreSQL',
    popular: true,
  },
  {
    location: DatabaseLocation.SelfHosted,
    engine: DatabaseEngine.PostgreSQL,
    name: 'Self Hosted PostgreSQL',
    popular: true,
  },
];
