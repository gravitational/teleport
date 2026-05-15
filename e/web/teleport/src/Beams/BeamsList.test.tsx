import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'e-teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';

import { BeamsList } from './BeamsList';

test('renders the My Beams placeholder with a link to the Quickstart', () => {
  render(
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContentMinWidth>
          <BeamsList />
        </ContentMinWidth>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );

  expect(
    screen.getByRole('heading', { level: 1, name: /My Beams/i })
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Viewing your Beams in-browser is coming soon/i)
  ).toBeInTheDocument();
  expect(screen.getByText('tsh beams ls')).toBeInTheDocument();

  const quickstartLink = screen.getByRole('link', {
    name: /Beams Quickstart/i,
  });
  expect(quickstartLink).toHaveAttribute('href', cfg.getBeamsQuickstartRoute());
});
