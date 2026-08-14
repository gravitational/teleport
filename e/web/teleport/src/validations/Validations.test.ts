import { isValidEmail } from 'e-teleport/validations/email';

describe('validations', () => {
  [
    // valid
    { email: 'char@char.char', valid: true },
    { email: 'char@char.char.char', valid: true },
    { email: 'char.char@char.char', valid: true },
    { email: 'char+char@char.char', valid: true },
    { email: 'char.char@char.char', valid: true },
    { email: 'char@char.char', valid: true },
    { email: 'char@char.char.char', valid: true },
    { email: 'char@123.123.123.123', valid: true },
    { email: 'char@[123.123.123.123]', valid: true },
    { email: '"char"@char.char', valid: true },
    { email: '1234567890@char.char', valid: true },
    { email: 'char@char-char.char', valid: true },
    { email: '_______@char.char', valid: true },
    { email: 'char-char@char.char', valid: true },
    // invalid
    { email: 'char@char@char.com', valid: false },
    { email: '@char', valid: false },
    { email: '@char.char', valid: false },
    { email: 'char@', valid: false },
    { email: 'char@char', valid: false },
    { email: 'char.com', valid: false },
    { email: 'char.@char', valid: false },
    { email: 'char@char..com', valid: false },
    { email: 'char', valid: false },
    { email: 'char.', valid: false },
    { email: '.char', valid: false },
    { email: '@.char', valid: false },
    { email: 'char@.', valid: false },
    { email: 'char@char.', valid: false },
    { email: '@char.char', valid: false },
    { email: 'char@.char', valid: false },
    { email: 'char.char.char', valid: false },
    { email: 'char@char@char.char', valid: false },
    { email: '#@%^%#$@#$@#.char', valid: false },
    // technically invalid but not enforced:
    { email: 'char.@char.com', valid: true },
    { email: '.char@char.com', valid: true },
    { email: 'char..char@char.char', valid: true },
    { email: 'char..char@char.char', valid: true },
    { email: 'char@char-char.char', valid: true },
  ].forEach(spec => {
    test(`email: ${spec.email}`, () => {
      const valid = isValidEmail(spec.email);
      expect(valid).toEqual(spec.valid);
    });
  });
});
