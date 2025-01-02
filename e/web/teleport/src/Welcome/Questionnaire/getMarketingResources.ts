import { ResourceOption } from 'e-teleport/Welcome/Questionnaire/types';
import { TermMatch } from 'teleport/Discover/SelectResource/getMarketingTermMatches';
import { MarketingParams } from 'teleport/services/userPreferences/types';

/**
 * Returns a list of resource options that match provided marketing parameters.
 *
 * @param marketingParams - MarketingParams from cluster user preferences which are set at signup
 * @returns an array of ResourceKeys associated with the marketing params for preselection in the onboarding survey
 *
 */
export const getMarketingResources = (
  marketingParams: MarketingParams
): ResourceOption[] | [] => {
  const params = [];
  if (marketingParams) {
    marketingParams.campaign && params.push(marketingParams.campaign);
    marketingParams.medium && params.push(marketingParams.medium);
    marketingParams.source && params.push(marketingParams.source);
    marketingParams.intent && params.push(marketingParams.intent);
  }
  if (params.length === 0) {
    return [];
  }

  const matches = new Set<ResourceOption>();
  params.forEach(p => {
    Object.values(TermMatch).forEach(m => {
      const resourceOption = matchLookup(m);
      const keyIndex = Object.values(ResourceOption).indexOf(resourceOption);
      const key = Object.keys(ResourceOption)[keyIndex] as ResourceOption;

      if (p.includes(m) && resourceOption) {
        matches.add(key);
      }
    });
  });

  return Array.from(matches);
};

const matchLookup = (m: string): ResourceOption => {
  switch (m) {
    case TermMatch.App:
      return ResourceOption.RESOURCE_WEB_APPLICATIONS;
    case TermMatch.Database:
      return ResourceOption.RESOURCE_DATABASES;
    case TermMatch.Kube:
    case TermMatch.Kubernetes:
    case TermMatch.K8s:
      return ResourceOption.RESOURCE_KUBERNETES;
    case TermMatch.SSH:
    case TermMatch.Server:
      return ResourceOption.RESOURCE_SERVER_SSH;
    case TermMatch.Desktop:
    case TermMatch.Windows:
      return ResourceOption.RESOURCE_WINDOWS_DESKTOPS;
    // currently we have no resource kind nor cluster resource defined for AWS
    // in the future, we can search the resources based on this term.
    case TermMatch.AWS:
    default:
      return null;
  }
};
