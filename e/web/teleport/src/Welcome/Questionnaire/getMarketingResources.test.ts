import { getMarketingResources } from 'e-teleport/Welcome/Questionnaire/getMarketingResources';
import { ResourceKey } from 'e-teleport/Welcome/Questionnaire/types';
import { MarketingParams } from 'teleport/services/userPreferences/types';

describe('getMarketingResources', () => {
  const testCases: {
    name: string;
    param: MarketingParams;
    expected: ResourceKey[];
  }[] = [
    {
      name: 'database matches RESOURCE_DATABASES & k8s matches RESOURCE_KUBERNETES',
      param: {
        campaign: 'foodatabasebar',
        source: 'k8ski',
        medium: '',
        intent: '',
      },
      expected: ['RESOURCE_DATABASES', 'RESOURCE_KUBERNETES'],
    },
    {
      name: 'app matches RESOURCE_WEB_APPLICATIONS',
      param: {
        campaign: '',
        source: 'baz',
        medium: '',
        intent: 'fooappbar',
      },
      expected: ['RESOURCE_WEB_APPLICATIONS'],
    },
    {
      name: 'windows matches RESOURCE_WINDOWS_DESKTOPS',
      param: {
        campaign: 'foowindowsbar',
        source: '',
        medium: '',
        intent: 'aws',
      },
      expected: ['RESOURCE_WINDOWS_DESKTOPS'],
    },
    {
      name: 'desktop matches RESOURCE_WINDOWS_DESKTOPS',
      param: {
        campaign: '',
        source: '',
        medium: 'foodesktopbar',
        intent: 'shoo',
      },
      expected: ['RESOURCE_WINDOWS_DESKTOPS'],
    },
    {
      name: 'ssh matches RESOURCE_SERVER_SSH',
      param: {
        campaign: '',
        source: 'foosshbar',
        medium: 'bar',
        intent: '',
      },
      expected: ['RESOURCE_SERVER_SSH'],
    },
    {
      name: 'server matches RESOURCE_SERVER_SSH',
      param: {
        campaign: 'fooserverbar',
        source: '',
        medium: '',
        intent: 'ser',
      },
      expected: ['RESOURCE_SERVER_SSH'],
    },
    {
      name: 'kube matches RESOURCE_KUBERNETES and windows matches RESOURCE_WINDOWS_DESKTOPS',
      param: {
        campaign: 'fookubebar',
        source: '',
        medium: 'windows',
        intent: '',
      },
      expected: ['RESOURCE_KUBERNETES', 'RESOURCE_WINDOWS_DESKTOPS'],
    },
    {
      name: 'kubernetes matches RESOURCE_KUBERNETES',
      param: {
        campaign: 'kubernetes',
        source: '',
        medium: '',
        intent: '',
      },
      expected: ['RESOURCE_KUBERNETES'],
    },
    {
      name: 'kube matches RESOURCE_KUBERNETES',
      param: {
        campaign: '',
        source: 'kube',
        medium: '',
        intent: '',
      },
      expected: ['RESOURCE_KUBERNETES'],
    },
    {
      name: 'k8s matches RESOURCE_KUBERNETES and ssh matches RESOURCE_SERVER_SSH',
      param: {
        campaign: '',
        source: '',
        medium: 'fook8sbar',
        intent: 'ssh',
      },
      expected: ['RESOURCE_KUBERNETES', 'RESOURCE_SERVER_SSH'],
    },
    {
      name: 'aws does not match',
      param: {
        campaign: 'fooaws',
        source: 'aws',
        medium: 'awshoo',
        intent: 'no aws',
      },
      expected: [],
    },
    {
      name: 'does not match when empty',
      param: {
        campaign: '',
        source: '',
        medium: '',
        intent: '',
      },
      expected: [],
    },
  ];
  test.each(testCases)('$name', testCase => {
    expect(getMarketingResources(testCase.param)).toEqual(testCase.expected);
  });
});
