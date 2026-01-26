import { DatabaseResourceAccess } from 'teleport/services/resources';

/**
 * Identities users can use/assume when connecting to a database.
 *
 * Derived from DatabaseResourceAccess with irrelevant fields
 * omitted and relevant fields required.
 */
export type DbIdentities = Required<
  Omit<DatabaseResourceAccess, 'db_labels' | 'db_service_labels' | 'db_roles'>
>;

export const emptyDbIdentities = (): DbIdentities => ({
  db_names: undefined,
  db_users: undefined,
});

/**
 * Array for iteration in switch statements.
 */
export const dbIdentities = Object.keys(emptyDbIdentities()) as Array<
  keyof DbIdentities
>;
