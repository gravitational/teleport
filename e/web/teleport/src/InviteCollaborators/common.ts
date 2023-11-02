import Logger from 'shared/libs/logger';

import { CloudUserInvites } from 'teleport/services/localStorage';

import { NotificationEntry } from './Notifications';

const logger = Logger.create('shared/hooks/useAttempt');

/**
 * Creates a notification entry for invite submit success.
 * @param recipients a list of recipients that were invited
 * @returns a notification entry
 */
export function createSuccessNotification(
  invite: CloudUserInvites
): NotificationEntry {
  let content: string;
  if (invite.recipients.length === 1) {
    content = `${invite.recipients[0]} was invited to your cluster`;
  } else {
    content = `${invite.recipients.length} members were invited to your cluster`;
  }

  return {
    content,
    severity: 'info',
    dismissAfterMs: 5000,
  };
}

/**
 * Creates an error notification to show when users could not be invited.
 * @param err
 * @returns a notification entry
 */
export function createErrorNotification(err: any): NotificationEntry {
  logger.error('could not invite users', err);
  return {
    content: `Could not invite users due to an error`,
    severity: 'error',
  };
}
