import { isValidEmail } from 'e-teleport/validations/email';
import { isValidPurchaseOrderPrefix } from 'e-teleport/validations/purchaseOrderPrefix';

describe('validations', () => {
  // eslint-disable-next-line jest/require-hook
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

  // eslint-disable-next-line jest/require-hook
  [
    { po: '123', valid: true },
    { po: 'abc', valid: true },
    { po: '123456789012', valid: true },
    { po: '123456789abc', valid: true },
    { po: '123435', valid: true },
    { po: '', valid: false },
    { po: '1', valid: false },
    { po: '12', valid: false },
    { po: '1234567890123', valid: false },
    { po: '12&', valid: false },
    { po: '12#', valid: false },
  ].forEach(spec => {
    test(`po: ${spec.po}`, () => {
      const valid = isValidPurchaseOrderPrefix(spec.po);
      expect(valid).toEqual(spec.valid);
    });
  });
});
