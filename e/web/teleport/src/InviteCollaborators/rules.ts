import { Option } from 'shared/components/Select';
import { Rule, requiredEmailLike } from 'shared/components/Validation/rules';

/**
 * A validator function to make sure all individual emails in a multi-select are
 * email-like, i.e. nonempty with content before and after the '@.
 * @param emails a list of entered email options
 * @returns a validator function that ensures all entered options are email-like
 */
export const requiredAllEmailLike: Rule<Option[]> =
  (emails: Option[]) => () => {
    if (!emails || emails.length === 0) {
      return {
        valid: false,
        message: 'At least one address is required',
      };
    }

    // We'll discard the actual message here - the only option other than
    // 'invalid' is 'empty', which is unlikely here, and is just as well written
    // as invalid in practice.
    let invalidEmails = [];
    for (let email of emails) {
      const valid = requiredEmailLike(email.value)();
      if (!valid.valid) {
        invalidEmails.push(email.value);
      }
    }

    if (invalidEmails.length > 0) {
      let message: string;
      if (invalidEmails.length === 1) {
        message = `Email is invalid: ${invalidEmails[0]}`;
      } else {
        message = `Emails are invalid: ${invalidEmails.join(', ')}`;
      }

      return {
        valid: false,
        message,
      };
    }

    return {
      valid: true,
    };
  };

/**
 * A validator function that ensures no entered usernames (emails) exist in a
 * set of existing usernames. The generated message aggregates all invalid
 * usernames into a user-friendly string.
 * @param existingUsers a set of existing usernames, converted to lowercase
 * @returns a validator function that returns invalid if any entered username
 *   already exists
 */
export const requiredAllUsersDoNotExist =
  (existingUsers: Set<string>): Rule<Option[]> =>
  (enteredUsers: Option[]) =>
  () => {
    let invalid: string[] = [];

    for (let user of enteredUsers) {
      let lower = user.value.toLowerCase();

      if (existingUsers.has(lower)) {
        invalid.push(user.value);
      }
    }

    if (invalid.length > 0) {
      let message: string;
      if (invalid.length === 1) {
        message = `User already exists: ${invalid[0]}`;
      } else {
        message = `Users already exist: ${invalid.join(', ')}`;
      }

      return {
        valid: false,
        message,
      };
    }

    return { valid: true };
  };

/**
 * A validator function that ensures a list of entered values contains no
 * duplicate usernames. The generated message aggregates all duplicate values
 * into a user-friendly string.
 * @returns a validator function that returns invalid if any usernames are
 * entered twice.
 */
export const requiredNoDuplicateUsers: Rule<Option[]> =
  (enteredUsers: Option[]) => () => {
    let invalid = new Set<string>();
    let seen = new Set<string>();

    for (let user of enteredUsers) {
      let lower = user.value.toLowerCase();
      if (seen.has(lower)) {
        invalid.add(lower);
      } else {
        seen.add(lower);
      }
    }

    if (invalid.size > 0) {
      const array = Array.from(invalid);

      let message;
      if (invalid.size === 1) {
        message = `Duplicate username: ${array[0]}`;
      } else {
        message = `Duplicate usernames: ${array.join(', ')}`;
      }

      return {
        valid: false,
        message,
      };
    }

    return { valid: true };
  };

/**
 * A rule function that combines multiple inner rule functions. All rules must
 * return `valid`, otherwise it returns a comma separated string containing all
 * invalid rule messages.
 * @param rules a list of rule functions to apply
 * @returns a rule function that ANDs all input rules
 */
export function requiredAll<T>(...rules: Rule<T>[]): Rule<T> {
  return (value: T) => () => {
    let messages = [];
    for (let r of rules) {
      let result = r(value)();
      if (!result.valid) {
        messages.push(result.message);
      }
    }

    if (messages.length > 0) {
      return {
        valid: false,
        message: messages.join('. '),
      };
    }

    return { valid: true };
  };
}

/**
 * A rule function that checks if the value exists at most `max` times within
 * an iterable. This allows for self-checks where `max = 1` if the iterable may
 * contain the value itself (but only once), or for uniqueness checks if
 * `max = 0`.
 *
 * This does not return a message and is only appropriate for e.g. error
 * highlighting.
 * @param entries an iterable of entries that the input value should not
 * duplicate too many times
 */
export function requiredMaxDuplicates<T>(
  entries: Iterable<T>,
  max: number = 0
): Rule<T> {
  return (value: T) => () => {
    // count all existing occurrences
    const counts = new Map<T, number>();
    for (let value of entries) {
      counts.set(value, (counts.get(value) || 0) + 1);
    }

    if (counts.has(value)) {
      // must be below threshold
      return { valid: counts.get(value) <= max };
    } else {
      // not in set, not duplicated
      return { valid: true };
    }
  };
}
