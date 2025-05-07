#ifndef SRC_REGEX_H_
#define SRC_REGEX_H_

#include <stdbool.h>
#include <stdio.h>

#ifdef USE_PCRE2
#include <pcre2.h>
#else
#include <pcre.h>
#endif


enum { REGEX_MATCH,
       REGEX_MATCH_PARTIAL,
       REGEX_NO_MATCH,
       REGEX_ERROR = -1,
};

struct regex_data;

#ifdef USE_PCRE2
struct regex_error_data {
	int error_code;
	PCRE2_SIZE error_offset;
};
#else
struct regex_error_data {
	char const *error_buffer;
	int error_offset;
};
#endif

struct mmap_area;

/**
 * regex_arch_string return a string that represents the pointer width, the
 * width of what the backend considers a size type, and the endianness of the
 * system that this library was build for. (e.g. for x86_64: "8-8-el").
 * This is required when loading stored regular espressions. PCRE2 regular
 * expressions are not portable across architectures that do not have a
 * matching arch-string.
 */
char const *regex_arch_string(void) ;

/**
 * regex_version returns the version string of the underlying regular
 * regular expressions library. In the case of PCRE it just returns the
 * result of pcre_version(). In the case of PCRE2, the very first time this
 * function is called it allocates a buffer large enough to hold the version
 * string and reads the PCRE2_CONFIG_VERSION option to fill the buffer.
 * The allocated buffer will linger in memory until the calling process is being
 * reaped.
 *
 * It may return NULL on error.
 */
char const *regex_version(void) ;
/**
 * This constructor function allocates a buffer for a regex_data structure.
 * The buffer is being initialized with zeroes.
 */
struct regex_data *regex_data_create(void) ;
/**
 * This complementary destructor function frees the a given regex_data buffer.
 * It also frees any non NULL member pointers with the appropriate pcreX_X_free
 * function. For PCRE this function respects the extra_owned field and frees
 * the pcre_extra data conditionally. Calling this function on a NULL pointer is
 * save.
 */
void regex_data_free(struct regex_data *regex) ;
/**
 * This function compiles the regular expression. Additionally, it prepares
 * data structures required by the different underlying engines. For PCRE
 * it calls pcre_study to generate optional data required for optimized
 * execution of the compiled pattern. In the case of PCRE2, it allocates
 * a pcre2_match_data structure of appropriate size to hold all possible
 * matches created by the pattern.
 *
 * @arg regex If successful, the structure returned through *regex was allocated
 *            with regex_data_create and must be freed with regex_data_free.
 * @arg pattern_string The pattern string that is to be compiled.
 * @arg errordata A pointer to a regex_error_data structure must be passed
 *                to this function. This structure depends on the underlying
 *                implementation. It can be passed to regex_format_error
 *                to generate a human readable error message.
 * @retval 0 on success
 * @retval -1 on error
 */
int regex_prepare_data(struct regex_data **regex, char const *pattern_string,
		       struct regex_error_data *errordata) ;
/**
 * This function loads a serialized precompiled pattern from a contiguous
 * data region given by map_area.
 *
 * @arg map_area Description of the memory region holding a serialized
 *               representation of the precompiled pattern.
 * @arg regex If successful, the structure returned through *regex was allocated
 *            with regex_data_create and must be freed with regex_data_free.
 * @arg do_load_precompregex If non-zero precompiled patterns get loaded from
 *			     the mmap region (ignored by PCRE1 back-end).
 * @arg regex_compiled Set to true if a precompiled pattern was loaded
 * 		       into regex, otherwise set to false to indicate later
 *		       compilation must occur
 *
 * @retval 0 on success
 * @retval -1 on error
 */
int regex_load_mmap(struct mmap_area *map_area,
		    struct regex_data **regex,
		    int do_load_precompregex,
		    bool *regex_compiled) ;
/**
 * This function stores a precompiled regular expression to a file.
 * In the case of PCRE, it just dumps the binary representation of the
 * precomplied pattern into a file. In the case of PCRE2, it uses the
 * serialization function provided by the library.
 *
 * @arg regex The precomplied regular expression data.
 * @arg fp A file stream specifying the output file.
 * @arg do_write_precompregex If non-zero precompiled patterns are written to
 *			      the output file (ignored by PCRE1 back-end).
 */
int regex_writef(struct regex_data *regex, FILE *fp,
		 int do_write_precompregex) ;
/**
 * This function applies a precompiled pattern to a subject string and
 * returns whether or not a match was found.
 *
 * @arg regex The precompiled pattern.
 * @arg subject The subject string.
 * @arg partial Boolean indicating if partial matches are wanted. A nonzero
 *              value is equivalent to specifying PCRE[2]_PARTIAL_SOFT as
 *              option to pcre_exec of pcre2_match.
 * @retval REGEX_MATCH if a match was found
 * @retval REGEX_MATCH_PARTIAL if a partial match was found
 * @retval REGEX_NO_MATCH if no match was found
 * @retval REGEX_ERROR if an error was encountered during the execution of the
 *                     regular expression
 */
int regex_match(struct regex_data *regex, char const *subject,
		int partial) ;
/**
 * This function compares two compiled regular expressions (regex1 and regex2).
 * It compares the binary representations of the compiled patterns. It is a very
 * crude approximation because the binary representation holds data like
 * reference counters, that has nothing to do with the actual state machine.
 *
 * @retval SELABEL_EQUAL if the pattern's binary representations are exactly
 *                       the same
 * @retval SELABEL_INCOMPARABLE otherwise
 */
int regex_cmp(struct regex_data *regex1, struct regex_data *regex2) ;
/**
 * This function takes the error data returned by regex_prepare_data and turns
 * it in to a human readable error message.
 * If the buffer given to hold the error message is to small it truncates the
 * message and indicates the truncation with an ellipsis ("...") at the end of
 * the buffer.
 *
 * @arg error_data Error data as returned by regex_prepare_data.
 * @arg buffer String buffer to hold the formatted error string.
 * @arg buf_size Total size of the given buffer in bytes.
 */
void regex_format_error(struct regex_error_data const *error_data, char *buffer,
			size_t buf_size) ;
#endif /* SRC_REGEX_H_ */
