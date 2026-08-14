import { getLabelFromInput } from './ResourceLabelInput';

test.each`
  input              | name         | value
  ${'foo: bar'}      | ${'foo'}     | ${'bar'}
  ${'foo: bar: baz'} | ${'foo'}     | ${'bar: baz'}
  ${'foo: bar:baz'}  | ${'foo'}     | ${'bar:baz'}
  ${'foo:bar: baz'}  | ${'foo:bar'} | ${'baz'}
  ${'foo:bar'}       | ${'foo'}     | ${'bar'}
  ${'foo : bar'}     | ${'foo'}     | ${'bar'}
  ${' foo:bar '}     | ${'foo'}     | ${'bar'}
`('getLabelFromInput parses "$input"', ({ input, name, value }) => {
  expect(getLabelFromInput(input)).toEqual({ name, value });
});
