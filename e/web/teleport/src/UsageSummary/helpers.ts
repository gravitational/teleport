/**
 *
 * @remarks
 * These helper methods are used in Cloud and are leveraged in this directory to
 * ensure a 1:1 view between Sales customer usage and Teleport customer usage.
 *
 */

export function usageUnixIsSeconds(unix: number): boolean {
  return Math.abs(Date.now() - unix) >= Math.abs(Date.now() - unix * 1000);
}

export function usageUnixInMilliseconds(unix: number): number {
  return usageUnixIsSeconds(unix) ? unix * 1000 : unix;
}
