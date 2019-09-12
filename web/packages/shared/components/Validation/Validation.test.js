import Validator from './Validation';

test('Validation', () => {
  const validator = new Validator();
  expect(validator.validate()).toEqual(true);
})