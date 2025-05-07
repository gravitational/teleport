#ifndef _SELINUX_CONTEXT_H_
#define _SELINUX_CONTEXT_H_

#ifdef __cplusplus
extern "C" {
#endif

/*
 * Functions to deal with security contexts in user space.
 */

	typedef struct {
		void *ptr;
	} context_s_t;

	typedef context_s_t *context_t;

/* Return a new context initialized to a context string */

	extern context_t context_new(const char *str);

/* 
 * Return a pointer to the string value of the context_t
 * Valid until the next call to context_str or context_free 
 * for the same context_t*
 */

	extern const char *context_str(context_t con);

/*
 * Return the string value of the context_t.
 * Similar to context_str(3), but the client owns the string
 * and needs to free it via free(3).
 */

	extern char *context_to_str(context_t con);

/* Free the storage used by a context */
	extern void context_free(context_t con);

/* Get a pointer to the string value of a context component */

	extern const char *context_type_get(context_t con);
	extern const char *context_range_get(context_t con);
	extern const char *context_role_get(context_t con);
	extern const char *context_user_get(context_t con);

/* Set a context component.  Returns nonzero if unsuccessful */

	extern int context_type_set(context_t con, const char *type);
	extern int context_range_set(context_t con, const char *range);
	extern int context_role_set(context_t con, const char *role);
	extern int context_user_set(context_t con, const char *user);

#ifdef __cplusplus
}
#endif
#endif
