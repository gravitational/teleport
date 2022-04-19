import { AWS_ACC_ID_REGEXP } from './Iam';

describe('test AWS account regex', () => {
  test.each`
    input              | expected
    ${'809732990878'}  | ${true}
    ${'a09732990878'}  | ${false}
    ${'809732a90878'}  | ${false}
    ${'80973299087a'}  | ${false}
    ${'80973299087'}   | ${false}
    ${'8097329908788'} | ${false}
    ${'1'}             | ${false}
    ${'123'}           | ${false}
    ${''}              | ${false}
    ${'twelvetwelve'}  | ${false}
  `(`$input should be $expected`, ({ input, expected }) => {
    const match = AWS_ACC_ID_REGEXP.test(input);
    expect(match).toBe(expected);
  });
});
