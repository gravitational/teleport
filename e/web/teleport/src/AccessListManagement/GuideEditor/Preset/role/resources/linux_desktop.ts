import { LinuxDesktopResourceAccess } from 'teleport/services/resources';

/**
 * Identities users can assume when connecting to a Linux desktop.
 *
 * Derived from LinuxDesktopResourceAccess with irrelevant fields
 * omitted and relevant fields required.
 */
export type LinuxDesktopIdentities = Required<
  Omit<LinuxDesktopResourceAccess, 'linux_desktop_labels'>
>;

export const emptyLinuxDesktopIdentities = (): LinuxDesktopIdentities => ({
  linux_desktop_logins: [],
});

/**
 * Array for iteration in switch statements.
 */
export const linuxDesktopIdentities = Object.keys(
  emptyLinuxDesktopIdentities()
) as Array<keyof LinuxDesktopIdentities>;
