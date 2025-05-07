#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "selinux_internal.h"
#include "context_internal.h"
#include <selinux/get_context_list.h>

/* context_menu - given a list of contexts, presents a menu of security contexts
 *            to the user.  Returns the number (position in the list) of
 *            the user selected context.
 */
static int context_menu(char ** list)
{
	int i;			/* array index                        */
	int choice = 0;		/* index of the user's choice         */
	char response[10];	/* string to hold the user's response */

	printf("\n\n");
	for (i = 0; list[i]; i++)
		printf("[%d] %s\n", i + 1, list[i]);

	while ((choice < 1) || (choice > i)) {
		printf("Enter number of choice: ");
		fflush(stdin);
		if (fgets(response, sizeof(response), stdin) == NULL)
			continue;
		fflush(stdin);
		choice = strtol(response, NULL, 10);
	}

	return (choice - 1);
}

/* query_user_context - given a list of context, allow the user to choose one.  The 
 *                  default is the first context in the list.  Returns 0 on
 *                  success, -1 on failure
 */
int query_user_context(char ** list, char ** usercon)
{
	char response[10];	/* The user's response                        */
	int choice;		/* The index in the list of the sid chosen by
				   the user                                   */

	if (!list[0])
		return -1;

	printf("\nYour default context is %s.\n", list[0]);
	if (list[1]) {
		printf("Do you want to choose a different one? [n]");
		fflush(stdin);
		if (fgets(response, sizeof(response), stdin) == NULL)
			return -1;
		fflush(stdin);

		if ((response[0] == 'y') || (response[0] == 'Y')) {
			choice = context_menu(list);
			*usercon = strdup(list[choice]);
			if (!(*usercon))
				return -1;
			return 0;
		}

		*usercon = strdup(list[0]);
		if (!(*usercon))
			return -1;
	} else {
		*usercon = strdup(list[0]);
		if (!(*usercon))
			return -1;
	}

	return 0;
}

/* get_field - given fieldstr - the "name" of a field, query the user 
 *             and set the new value of the field
 */
static void get_field(const char *fieldstr, char *newfield, int newfieldlen)
{
	int done = 0;		/* true if a non-empty field has been obtained */

	while (!done) {		/* Keep going until we get a value for the field */
		printf("\tEnter %s ", fieldstr);
		fflush(stdin);
		if (fgets(newfield, newfieldlen, stdin) == NULL)
			continue;
		fflush(stdin);
		if (newfield[strlen(newfield) - 1] == '\n')
			newfield[strlen(newfield) - 1] = '\0';

		if (strlen(newfield) == 0) {
			printf("You must enter a %s\n", fieldstr);
		} else {
			done = 1;
		}
	}
}

/* manual_user_enter_context - provides a way for a user to manually enter a
 *                     context in case the policy doesn't allow a list
 *                     to be obtained.
 *                     given the userid, queries the user and places the
 *                     context chosen by the user into usercon.  Returns 0
 *                     on success.
 */
int manual_user_enter_context(const char *user, char ** newcon)
{
	char response[10];	/* Used to get yes or no answers from user */
	char role[100];		/* The role requested by the user          */
	int rolelen = 100;
	char type[100];		/* The type requested by the user          */
	int typelen = 100;
	char level[100];	/* The level requested by the user         */
	int levellen = 100;
	int mls_enabled = is_selinux_mls_enabled();

	context_t new_context;	/* The new context chosen by the user     */
	const char *user_context = NULL;	/* String value of the user's context     */
	int done = 0;		/* true if a valid sid has been obtained  */

	/* Initialize the context.  How this is done depends on whether
	   or not MLS is enabled                                        */
	if (mls_enabled)
		new_context = context_new("user:role:type:level");
	else
		new_context = context_new("user:role:type");

	if (!new_context)
		return -1;

	while (!done) {
		printf("Would you like to enter a security context? [y]");
		if (fgets(response, sizeof(response), stdin) == NULL
		    || (response[0] == 'n') || (response[0] == 'N')) {
			context_free(new_context);
			return -1;
		}

		/* Allow the user to enter each field of the context individually */
		if (context_user_set(new_context, user)) {
			context_free(new_context);
			return -1;
		}
		get_field("role", role, rolelen);
		if (context_role_set(new_context, role)) {
			context_free(new_context);
			return -1;
		}
		get_field("type", type, typelen);
		if (context_type_set(new_context, type)) {
			context_free(new_context);
			return -1;
		}

		if (mls_enabled) {
			get_field("level", level, levellen);
			if (context_range_set(new_context, level)) {
				context_free(new_context);
				return -1;
			}
		}

		/* Get the string value of the context and see if it is valid. */
		user_context = context_str(new_context);
		if (!user_context) {
			context_free(new_context);
			return -1;
		}
		if (!security_check_context(user_context))
			done = 1;
		else
			printf("Not a valid security context\n");
	}

	*newcon = strdup(user_context);
	context_free(new_context);
	if (!(*newcon))
		return -1;
	return 0;
}
