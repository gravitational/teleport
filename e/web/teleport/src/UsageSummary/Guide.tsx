import { Box } from 'design';
import {
  InfoParagraph,
  InfoTitle,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';

export function Guide() {
  return (
    <Box>
      <InfoTitle>How Monthly Active Users (MAU) are Calculated</InfoTitle>
      <InfoParagraph>
        Monthly Active Users (MAU) is the aggregate number of unique active
        users accessing Teleport during each monthly cycle.
      </InfoParagraph>
      <InfoTitle>
        How Teleport Protected Resources (TPR) are Calculated
      </InfoTitle>
      <InfoParagraph>
        Teleport Protected Resources (TPR) is an averaged aggregate number of
        unique resources connected to Teleport.
      </InfoParagraph>
      <InfoParagraph>
        A &quot;resource&quot; is any unique bot, such as a CI/CD Jenkins or
        GitHub Actions job, or a distinct computing resource, including a
        Kubernetes cluster, SSH server, database instance, or serverless
        endpoint, that registers with the Teleport cluster at least once a
        month.
      </InfoParagraph>
      <InfoParagraph>
        TPR is calculated by aggregating the total number of unique resources
        during the span of each hour in the day, and averaging the hourly count
        to create a daily TPR. The daily TPRs are then averaged across each
        cycle.
      </InfoParagraph>
      <InfoTitle>
        How Machine and Workload Identities (MWI) are Calculated
      </InfoTitle>
      <InfoParagraph>
        Machine and Workload Identities is an aggregate number of bots, bot
        instances, and SPIFFE IDs.
      </InfoParagraph>
      <InfoParagraph>
        MWIs are calculated by counting the total number of bots, bot instances,
        and unique SPIFFE IDs seen in an hour and averaging the hourly number to
        create a daily average. The daily MWI numbers are then averaged across
        each cycle.
      </InfoParagraph>
      <ReferenceLinks
        links={[
          {
            title: 'Usage Reporting and Billing',
            href: 'https://goteleport.com/docs/usage-billing/',
          },
        ]}
      />
    </Box>
  );
}
