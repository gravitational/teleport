import { AttemptStatus } from 'shared/hooks/useAsync';

/**
 *  `getEmptyTableText` returns text to be used in an async resource table
 *
 *  @example
 *  // Successfully fetched with zero results returned
 *  getEmptyTableText(fetchAttempt.status, "servers"); // "No servers found"
 *
 *  @param status - AttemptStatus from a useAsync request
 *  @param pluralResourceNoun - String that represents the plural of a resource, i.e. "servers", "databases"
 */
export function getEmptyTableText(
  status: AttemptStatus,
  pluralResourceNoun: string
) {
  switch (status) {
    case 'error':
      return `Failed to fetch ${pluralResourceNoun}.`;
    case '':
      return 'Searching…';
    case 'processing':
      return 'Searching…';
    case 'success':
      return `No ${pluralResourceNoun} found.`;
  }
}
