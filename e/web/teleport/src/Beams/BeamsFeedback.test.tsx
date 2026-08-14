import { render, screen } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'e-teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';

import { BeamsFeedback } from './BeamsFeedback';

function renderBeamsFeedback() {
  render(
    <InfoGuidePanelProvider>
      <ContentMinWidth>
        <BeamsFeedback />
      </ContentMinWidth>
    </InfoGuidePanelProvider>
  );
}

test('renders the scheduler card with heading, body, and link', () => {
  renderBeamsFeedback();

  expect(
    screen.getByRole('heading', {
      name: /14-day trial of Beams, powered by Teleport/i,
    })
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Extend your trial or discuss commercial options/i)
  ).toBeInTheDocument();

  const schedulerLink = screen.getByRole('link', {
    name: /Schedule time with a Teleporter/i,
  });
  expect(schedulerLink).toHaveAttribute('href', cfg.beamsSchedulerUrl);
  expect(schedulerLink).toHaveAttribute('target', '_blank');
  expect(schedulerLink).toHaveAttribute('rel', 'noopener noreferrer');
});

test('renders the Slack card with body and link', () => {
  renderBeamsFeedback();

  expect(screen.getByText(/#beams/)).toBeInTheDocument();

  const slackLink = screen.getByRole('link', {
    name: /Join Teleport on Slack/i,
  });
  expect(slackLink).toHaveAttribute('href', cfg.communitySlackUrl);
  expect(slackLink).toHaveAttribute('target', '_blank');
  expect(slackLink).toHaveAttribute('rel', 'noopener noreferrer');
});
