#ifndef SELINUX_INTERNAL_H_
#define SELINUX_INTERNAL_H_

#include <selinux/selinux.h>
#include <errno.h>
#include <pthread.h>
#include <stdio.h>


extern int require_seusers ;
extern int selinux_page_size ;

/* Make pthread_once optional */
#pragma weak pthread_once
#pragma weak pthread_key_create
#pragma weak pthread_key_delete
#pragma weak pthread_setspecific

/* Call handler iff the first call.  */
#define __selinux_once(ONCE_CONTROL, INIT_FUNCTION)	\
	do {						\
		if (pthread_once != NULL)		\
			pthread_once (&(ONCE_CONTROL), (INIT_FUNCTION));  \
		else if ((ONCE_CONTROL) == PTHREAD_ONCE_INIT) {		  \
			INIT_FUNCTION ();		\
			(ONCE_CONTROL) = 2;		\
		}					\
	} while (0)

/* Pthread key macros */
#define __selinux_key_create(KEY, DESTRUCTOR)			\
	(pthread_key_create != NULL ? pthread_key_create(KEY, DESTRUCTOR) : -1)

#define __selinux_key_delete(KEY)				\
	do {							\
		if (pthread_key_delete != NULL)			\
			pthread_key_delete(KEY);		\
	} while (0)

#define __selinux_setspecific(KEY, VALUE)			\
	do {							\
		if (pthread_setspecific != NULL)		\
			pthread_setspecific(KEY, VALUE);	\
	} while (0)

/* selabel_lookup() is only thread safe if we're compiled with pthreads */

#pragma weak pthread_mutex_init
#pragma weak pthread_mutex_destroy
#pragma weak pthread_mutex_lock
#pragma weak pthread_mutex_unlock

#define __pthread_mutex_init(LOCK, ATTR) 			\
	do {							\
		if (pthread_mutex_init != NULL)			\
			pthread_mutex_init(LOCK, ATTR);		\
	} while (0)

#define __pthread_mutex_destroy(LOCK) 				\
	do {							\
		if (pthread_mutex_destroy != NULL)		\
			pthread_mutex_destroy(LOCK);		\
	} while (0)

#define __pthread_mutex_lock(LOCK) 				\
	do {							\
		if (pthread_mutex_lock != NULL)			\
			pthread_mutex_lock(LOCK);		\
	} while (0)

#define __pthread_mutex_unlock(LOCK) 				\
	do {							\
		if (pthread_mutex_unlock != NULL)		\
			pthread_mutex_unlock(LOCK);		\
	} while (0)

#pragma weak pthread_create
#pragma weak pthread_join
#pragma weak pthread_cond_init
#pragma weak pthread_cond_signal
#pragma weak pthread_cond_destroy
#pragma weak pthread_cond_wait

/* check if all functions needed to do parallel operations are available */
#define __pthread_supported (					\
	pthread_create &&					\
	pthread_join &&						\
	pthread_cond_init &&					\
	pthread_cond_destroy &&					\
	pthread_cond_signal &&					\
	pthread_cond_wait					\
)

#define SELINUXDIR "/etc/selinux/"
#define SELINUXCONFIG SELINUXDIR "config"

extern int has_selinux_config ;

#ifndef HAVE_STRLCPY
size_t strlcpy(char *dest, const char *src, size_t size);
#endif

#ifndef HAVE_REALLOCARRAY
void *reallocarray(void *ptr, size_t nmemb, size_t size);
#endif

/* Use to ignore intentional unsigned under- and overflows while running under UBSAN. */
#if defined(__clang__) && defined(__clang_major__) && (__clang_major__ >= 4)
#if (__clang_major__ >= 12)
#define ignore_unsigned_overflow_        __attribute__((no_sanitize("unsigned-integer-overflow", "unsigned-shift-base")))
#else
#define ignore_unsigned_overflow_        __attribute__((no_sanitize("unsigned-integer-overflow")))
#endif
#else
#define ignore_unsigned_overflow_
#endif

/* Ignore usage of deprecated declaration */
#ifdef __clang__
#define IGNORE_DEPRECATED_DECLARATION_BEGIN \
	_Pragma("clang diagnostic push") \
	_Pragma("clang diagnostic ignored \"-Wdeprecated-declarations\"")
#define IGNORE_DEPRECATED_DECLARATION_END \
	_Pragma("clang diagnostic pop")
#elif defined __GNUC__
#define IGNORE_DEPRECATED_DECLARATION_BEGIN \
	_Pragma("GCC diagnostic push") \
	_Pragma("GCC diagnostic ignored \"-Wdeprecated-declarations\"")
#define IGNORE_DEPRECATED_DECLARATION_END \
	_Pragma("GCC diagnostic pop")
#else
#define IGNORE_DEPRECATED_DECLARATION_BEGIN
#define IGNORE_DEPRECATED_DECLARATION_END
#endif

static inline void fclose_errno_safe(FILE *stream)
{
	int saved_errno;

	saved_errno = errno;
	(void) fclose(stream);
	errno = saved_errno;
}

#ifdef __GNUC__
# define likely(x)			__builtin_expect(!!(x), 1)
# define unlikely(x)			__builtin_expect(!!(x), 0)
#else
# define likely(x)			(x)
# define unlikely(x)			(x)
#endif /* __GNUC__ */

#endif /* SELINUX_INTERNAL_H_ */
