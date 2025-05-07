/*
 * This file describes the class and permission mappings used to 
 * hide the kernel numbers from userspace by allowing userspace object
 * managers to specify a list of classes and permissions.
 */
#ifndef _SELINUX_MAPPING_H_
#define _SELINUX_MAPPING_H_

#include <selinux/selinux.h>

/*
 * Get real, kernel values from mapped values
 */

extern security_class_t
unmap_class(security_class_t tclass);

extern access_vector_t
unmap_perm(security_class_t tclass, access_vector_t tperm);

/*
 * Get mapped values from real, kernel values
 */

extern security_class_t
map_class(security_class_t kclass);

extern access_vector_t
map_perm(security_class_t tclass, access_vector_t kperm);

extern void
map_decision(security_class_t tclass, struct av_decision *avd);

#endif				/* _SELINUX_MAPPING_H_ */
