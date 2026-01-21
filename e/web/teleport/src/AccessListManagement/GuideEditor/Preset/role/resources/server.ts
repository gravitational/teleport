import { ServerResourceAccess } from 'teleport/services/resources';

/**
 * Identities users can assume when connecting to a server.
 *
 * Derived from ServerResourceAccess with irrelevant fields
 * omitted and relevant fields required.
 */
export type ServerIdentities = Required<
  Omit<ServerResourceAccess, 'node_labels'>
>;

export const emptyServerIdentities = (): ServerIdentities => ({
  logins: [],
});
