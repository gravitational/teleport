/* oxlint-disable jest/no-conditional-expect */
import { Option } from 'shared/components/Select';
import { requiredAll, Rule } from 'shared/components/Validation/rules';

import {
  requiredAllEmailLike,
  requiredAllUsersDoNotExist,
  requiredMaxDuplicates,
  requiredNoDuplicateUsers,
} from './rules';

describe('requiredAllEmailLike', () => {
  test.each`
    emails                     | valid    | contains
    ${['alice@example.com']}   | ${true}  | ${null}
    ${['a@b', 'c@d']}          | ${true}  | ${null}
    ${['a@b', 'c@d', 'e']}     | ${false} | ${'Email is invalid'}
    ${['a@b', 'c@d', 'e@f@g']} | ${false} | ${'Email is invalid'}
    ${['a@b', 'c@d', '']}      | ${false} | ${'Email is invalid'}
    ${['a@b', 'a', 'b', 'c']}  | ${false} | ${'Emails are invalid'}
  `('emails: $emails', ({ emails, valid, contains }) => {
    const options: Option[] = emails.map(e => ({
      value: e,
      label: e,
    }));
    const result = requiredAllEmailLike(options)();

    expect(result.valid).toEqual(valid);

    if (contains) {
      expect(result.message).not.toBeNull();
      expect(result.message).toContain(contains);
    }
  });
});

describe('requiredAllUsersDoNotExist', () => {
  const existing = new Set(['alice@example.com', 'bob@example.com']);

  test.each`
    entered                                         | valid    | contains
    ${['charlie@example.com']}                      | ${true}  | ${null}
    ${['a', 'b', 'c']}                              | ${true}  | ${null}
    ${['a', 'a', 'a']}                              | ${true}  | ${null}
    ${['alice@example.com']}                        | ${false} | ${'User already exists'}
    ${['Alice@example.com']}                        | ${false} | ${'User already exists'}
    ${['alice@example.com', 'charlie@example.com']} | ${false} | ${'User already exists'}
    ${['alice@example.com', 'bob@example.com']}     | ${false} | ${'Users already exist'}
  `('users: $entered', ({ entered, valid, contains }) => {
    const options: Option[] = entered.map(e => ({
      value: e,
      label: e,
    }));

    const result = requiredAllUsersDoNotExist(existing)(options)();
    expect(result.valid).toEqual(valid);

    if (contains) {
      expect(result.message).not.toBeNull();
      expect(result.message).toContain(contains);
    }
  });
});

describe('requiredNoDuplicateUsers', () => {
  test.each`
    entered                      | valid    | contains
    ${['a']}                     | ${true}  | ${null}
    ${['a', 'b', 'c']}           | ${true}  | ${null}
    ${['a', 'a']}                | ${false} | ${'Duplicate username:'}
    ${['a', 'A']}                | ${false} | ${'Duplicate username:'}
    ${['a', 'a', 'b', 'b', 'b']} | ${false} | ${'Duplicate usernames:'}
  `('duplicate users: $entered', ({ entered, valid, contains }) => {
    const options: Option[] = entered.map(e => ({
      value: e,
      label: e,
    }));

    const result = requiredNoDuplicateUsers(options)();
    expect(result.valid).toEqual(valid);

    if (contains) {
      expect(result.message).not.toBeNull();
      expect(result.message).toContain(contains);
    }
  });
});

describe('requiredMaxDuplicates', () => {
  const entries = ['a', 'b', 'c', 'd'];

  test.each`
    value  | max  | valid
    ${'e'} | ${0} | ${true}
    ${'a'} | ${1} | ${true}
    ${'a'} | ${0} | ${false}
  `('max duplicates: value=$value, max=$max', ({ value, max, valid }) => {
    const result = requiredMaxDuplicates(entries, max)(value)();
    expect(result.valid).toEqual(valid);
  });
});

describe('requiredAll', () => {
  test.each`
    inputs                       | valid    | message
    ${[]}                        | ${true}  | ${null}
    ${[null]}                    | ${true}  | ${null}
    ${[null, null, null]}        | ${true}  | ${null}
    ${['a']}                     | ${false} | ${'a'}
    ${['a', 'a']}                | ${false} | ${'a. a'}
    ${['a', 'a', 'b', 'b', 'b']} | ${false} | ${'a. a. b. b. b'}
  `('requiredAll: $inputs', ({ inputs, valid, message }) => {
    const fns: Rule<Option>[] = inputs.map(e => () => {
      return () => {
        if (e) {
          return { valid: false, message: e };
        }
        return { valid: true };
      };
    });

    const result = requiredAll<Option>(...fns)({
      value: 'foo',
      label: 'foo',
    })();

    expect(result.valid).toEqual(valid);

    if (message) {
      expect(result.message).not.toBeNull();
      expect(result.message).toEqual(message);
    }
  });
});
