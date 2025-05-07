#ifndef _POLICY_H_
#define _POLICY_H_

/* Private definitions used internally by libselinux. */

/*
 * xattr name for SELinux attributes.
 * This may have been exported via Kernel uapi header.
 */
#ifndef XATTR_NAME_SELINUX
#define XATTR_NAME_SELINUX "security.selinux"
#endif

/* Initial length guess for getting contexts. */
#define INITCONTEXTLEN 255

/* selinux file system type */
#define SELINUXFS "selinuxfs"

/* selinuxfs magic number */
#define SELINUX_MAGIC 0xf97cff8c

/* Preferred selinux mount location */
#define SELINUXMNT "/sys/fs/selinux"
#define OLDSELINUXMNT "/selinux"

/* selinuxfs mount point */
extern char *selinux_mnt;

#define FILECONTEXTS "/etc/security/selinux/file_contexts"

#define DEFAULT_POLICY_VERSION 15

#endif
