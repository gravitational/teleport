#include "selinux_internal.h"

#include <errno.h>
#include <stdlib.h>
#include <string.h>


#ifndef HAVE_STRLCPY
size_t strlcpy(char *dest, const char *src, size_t size)
{
	size_t ret = strlen(src);

	if (size) {
		size_t len = (ret >= size) ? size - 1 : ret;
		memcpy(dest, src, len);
		dest[len] = '\0';
	}
	return ret;
}
#endif /* HAVE_STRLCPY */

#ifndef HAVE_REALLOCARRAY
void *reallocarray(void *ptr, size_t nmemb, size_t size)
{
	if (size && nmemb > SIZE_MAX / size) {
		errno = ENOMEM;
		return NULL;
	}

	return realloc(ptr, nmemb * size);
}
#endif /* HAVE_REALLOCARRAY */
