import { parseGitHubUrl } from './ConnectGitHub'

describe('parseGitHubUrl', () => {
  const testCases = [
    {
      url: 'https://github.com/gravitational/teleport',
      expected: { repositoryOwner: 'gravitational', repository: 'teleport' },
      shouldThrow: false,
    },
    {
      url: 'github.com/gravitational/teleport',
      expected: { repositoryOwner: '', repository: '' },
      shouldThrow: true,
    },
    {
      url: 'www.example.com/company/project',
      expected: { repositoryOwner: 'company', repository: 'project' },
      shouldThrow: false,
    }
  ];
  test.each(testCases)(
    'should return repo="$expected.repository" and owner="$expected.repositoryOwner" and throw=$shouldThrow for url=$url',
    ({ url, expected, shouldThrow }) => {
      if (shouldThrow) {
        try {
          const { repositoryOwner, repository } = parseGitHubUrl(url)
          expect(repositoryOwner).toBe(expected.repositoryOwner)
          expect(repository).toBe(expected.repository)
        } catch (err) {
          shouldThrow ? expect(err).not.toBe(null) : expect(err).toBe(null)
        }
        return
      }
    }
  );
})