import { WindowsDesktopResourceAccess } from 'teleport/services/resources';

/**
 * Identities users can assume when connecting to a Windows desktop.
 *
 * Derived from WindowsDesktopResourceAccess with irrelevant fields
 * omitted and relevant fields required.
 */
export type DesktopIdentities = Required<
  Omit<WindowsDesktopResourceAccess, 'windows_desktop_labels'>
>;

export const emptyDesktopIdentities = (): DesktopIdentities => ({
  windows_desktop_logins: [],
});

/**
 * Array for iteration in switch statements.
 */
export const desktopIdentities = Object.keys(emptyDesktopIdentities()) as Array<
  keyof DesktopIdentities
>;
