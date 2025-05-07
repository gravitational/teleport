/*
 * Class and permission mappings.
 */

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <stdbool.h>
#include <selinux/selinux.h>
#include <selinux/avc.h>
#include "callbacks.h"
#include "mapping.h"
#include "selinux_internal.h"

/*
 * Class and permission mappings
 */

struct selinux_mapping {
	security_class_t value; /* real, kernel value */
	unsigned num_perms;
	access_vector_t perms[sizeof(access_vector_t) * 8];
};

static struct selinux_mapping *current_mapping = NULL;
static security_class_t current_mapping_size = 0;

/*
 * Mapping setting function
 */

int
selinux_set_mapping(const struct security_class_mapping *map)
{
	size_t size = sizeof(struct selinux_mapping);
	security_class_t i, j;
	unsigned k;
	bool print_unknown_handle = false;
	bool reject = (security_reject_unknown() == 1);
	bool deny = (security_deny_unknown() == 1);

	free(current_mapping);
	current_mapping = NULL;
	current_mapping_size = 0;

	if (avc_reset() < 0)
		goto err;

	/* Find number of classes in the input mapping */
	if (!map) {
		errno = EINVAL;
		goto err;
	}
	i = 0;
	while (map[i].name)
		i++;

	/* Allocate space for the class records, plus one for class zero */
	current_mapping = (struct selinux_mapping *)calloc(++i, size);
	if (!current_mapping)
		goto err;

	/* Store the raw class and permission values */
	j = 0;
	while (map[j].name) {
		const struct security_class_mapping *p_in = map + (j++);
		struct selinux_mapping *p_out = current_mapping + j;

		p_out->value = string_to_security_class(p_in->name);
		if (!p_out->value) {
			selinux_log(SELINUX_INFO,
				    "SELinux: Class %s not defined in policy.\n",
				    p_in->name);
			if (reject)
				goto err2;
			p_out->num_perms = 0;
			print_unknown_handle = true;
			continue;
		}

		k = 0;
		while (p_in->perms[k]) {
			/* An empty permission string skips ahead */
			if (!*p_in->perms[k]) {
				k++;
				continue;
			}
			p_out->perms[k] = string_to_av_perm(p_out->value,
							    p_in->perms[k]);
			if (!p_out->perms[k]) {
				selinux_log(SELINUX_INFO,
					    "SELinux:  Permission %s in class %s not defined in policy.\n",
					    p_in->perms[k], p_in->name);
				if (reject)
					goto err2;
				print_unknown_handle = true;
			}
			k++;
		}
		p_out->num_perms = k;
	}

	if (print_unknown_handle)
		selinux_log(SELINUX_INFO,
			    "SELinux: the above unknown classes and permissions will be %s\n",
			    deny ? "denied" : "allowed");

	/* Set the mapping size here so the above lookups are "raw" */
	current_mapping_size = i;
	return 0;
err2:
	free(current_mapping);
	current_mapping = NULL;
	current_mapping_size = 0;
err:
	return -1;
}

/*
 * Get real, kernel values from mapped values
 */

security_class_t
unmap_class(security_class_t tclass)
{
	if (tclass < current_mapping_size)
		return current_mapping[tclass].value;

	/* If here no mapping set or the class requested is not valid. */
	if (current_mapping_size != 0) {
		errno = EINVAL;
		return 0;
	}
	else
		return tclass;
}

access_vector_t
unmap_perm(security_class_t tclass, access_vector_t tperm)
{
	if (tclass < current_mapping_size) {
		unsigned i;
		access_vector_t kperm = 0;

		for (i = 0; i < current_mapping[tclass].num_perms; i++)
			if (tperm & (UINT32_C(1)<<i)) {
				kperm |= current_mapping[tclass].perms[i];
				tperm &= ~(UINT32_C(1)<<i);
			}
		return kperm;
	}

	/* If here no mapping set or the perm requested is not valid. */
	if (current_mapping_size != 0) {
		errno = EINVAL;
		return 0;
	}
	else
		return tperm;
}

/*
 * Get mapped values from real, kernel values
 */

security_class_t
map_class(security_class_t kclass)
{
	security_class_t i;

	for (i = 0; i < current_mapping_size; i++)
		if (current_mapping[i].value == kclass)
			return i;

/* If here no mapping set or the class requested is not valid. */
	if (current_mapping_size != 0) {
		errno = EINVAL;
		return 0;
	}
	else
		return kclass;
}

access_vector_t
map_perm(security_class_t tclass, access_vector_t kperm)
{
	if (tclass < current_mapping_size) {
		unsigned i;
		access_vector_t tperm = 0;

		for (i = 0; i < current_mapping[tclass].num_perms; i++)
			if (kperm & current_mapping[tclass].perms[i]) {
				tperm |= UINT32_C(1)<<i;
				kperm &= ~current_mapping[tclass].perms[i];
			}

		if (tperm == 0) {
			errno = EINVAL;
			return 0;
		}
		else
			return tperm;
	}
	return kperm;
}

void
map_decision(security_class_t tclass, struct av_decision *avd)
{
	if (tclass < current_mapping_size) {
		bool allow_unknown = (security_deny_unknown() == 0);
		struct selinux_mapping *mapping = &current_mapping[tclass];
		unsigned int i, n = mapping->num_perms;
		access_vector_t result;

		for (i = 0, result = 0; i < n; i++) {
			if (avd->allowed & mapping->perms[i])
				result |= UINT32_C(1)<<i;
			else if (allow_unknown && !mapping->perms[i])
				result |= UINT32_C(1)<<i;
		}
		avd->allowed = result;

		for (i = 0, result = 0; i < n; i++) {
			if (avd->decided & mapping->perms[i])
				result |= UINT32_C(1)<<i;
			else if (allow_unknown && !mapping->perms[i])
				result |= UINT32_C(1)<<i;
		}
		avd->decided = result;

		for (i = 0, result = 0; i < n; i++)
			if (avd->auditallow & mapping->perms[i])
				result |= UINT32_C(1)<<i;
		avd->auditallow = result;

		for (i = 0, result = 0; i < n; i++) {
			if (avd->auditdeny & mapping->perms[i])
				result |= UINT32_C(1)<<i;
			else if (!allow_unknown && !mapping->perms[i])
				result |= UINT32_C(1)<<i;
		}

		/*
		 * Make sure we audit denials for any permission check
		 * beyond the mapping->num_perms since this indicates
		 * a bug in the object manager.
		 */
		for (; i < (sizeof(result)*8); i++)
			result |= UINT32_C(1)<<i;
		avd->auditdeny = result;
	}
}
