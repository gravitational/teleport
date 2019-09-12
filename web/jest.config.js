module.exports = {
  moduleNameMapper: {
    'shared/(.*)$': '<rootDir>/packages/shared/$1',
    'design/(.*)$': '<rootDir>/packages/design/src/$1',
    'gravity/(.*)$': '<rootDir>/packages/gravity/src/$1',
    'teleport/(.*)$': '<rootDir>/packages/teleport/src/$1',
    'e-teleport/(.*)$': '<rootDir>/packages/e/teleport/src/$1',
    'e-shared/(.*)$': '<rootDir>/packages/e/shared/$1',
    'e-gravity/(.*)$': '<rootDir>/packages/e/gravity/$1',
  },
};
