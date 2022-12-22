export type DbType = 'redshift' | 'rds' | 'gcp' | 'self-hosted';

export type DbProtocol =
  | 'postgres'
  | 'mysql'
  | 'mongodb'
  | 'sqlserver'
  | 'redis';

const formatProtocol = (input: DbProtocol) => {
  switch (input) {
    case 'postgres':
      return 'PostgreSQL';
    case 'mysql':
      return 'MySQL/MariaDB';
    case 'mongodb':
      return 'MongoDB';
    case 'sqlserver':
      return 'SQL Server';
    case 'redis':
      return 'Redis';
    default:
      return input;
  }
};

export const formatDatabaseInfo = (type: DbType, protocol: DbProtocol) => {
  const output = { type, protocol, title: '' };

  switch (type) {
    case 'rds':
      output.title = `RDS ${formatProtocol(protocol)}`;
      return output;
    case 'redshift':
      output.title = 'Redshift';
      return output;
    case 'self-hosted':
      output.title = `Self-hosted ${formatProtocol(protocol)}`;
      return output;
    case 'gcp':
      output.title = `Cloud SQL ${formatProtocol(protocol)}`;
      return output;
    default:
      output.title = `${type} ${formatProtocol(protocol)}`;
      return output;
  }
};

export type DatabaseInfo = ReturnType<typeof formatDatabaseInfo>;
