import userEvent from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';

import { ContentMinWidth } from 'teleport/Main/Main';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { BeamsQuickstart } from '.';

function renderPage() {
  return render(
    <TeleportProviderBasic>
      <ContentMinWidth>
        <BeamsQuickstart />
      </ContentMinWidth>
    </TeleportProviderBasic>
  );
}

test('renders every section title from the TOC', () => {
  renderPage();

  expect(
    screen.getByRole('heading', { name: /Check your client setup/i })
  ).toBeInTheDocument();
  expect(
    screen.getByRole('heading', { name: /Run commands in the background/i })
  ).toBeInTheDocument();
  expect(
    screen.getByRole('heading', { name: /Vibe code & publish an app/i })
  ).toBeInTheDocument();
  expect(
    screen.getByRole('heading', { name: /Find more examples in the CLI/i })
  ).toBeInTheDocument();
  expect(
    screen.getByRole('heading', { name: /Docs & FAQs/i })
  ).toBeInTheDocument();
});

test('appends --auth=passwordless when the passwordless radio is selected', async () => {
  const user = userEvent.setup();
  renderPage();

  expect(document.body).not.toHaveTextContent(/--auth=passwordless/);

  await user.click(screen.getByLabelText(/passwordless \(passkeys\) set up/i));

  expect(document.body).toHaveTextContent(/--auth=passwordless/);
});

test('reveals and hides the install command via Show/Hide command. button', async () => {
  const user = userEvent.setup();
  renderPage();

  const installCmd = /cdn\.teleport\.dev|scripts\/install\.sh/;
  expect(document.body).not.toHaveTextContent(installCmd);

  await user.click(screen.getByRole('button', { name: /Show command\./i }));
  expect(document.body).toHaveTextContent(installCmd);

  await user.click(screen.getByRole('button', { name: /Hide command\./i }));
  expect(document.body).not.toHaveTextContent(installCmd);
});

test('FAQ answers are hidden until a question is expanded', async () => {
  const user = userEvent.setup();
  renderPage();

  expect(screen.getByText(/How do I find my Beam.s ID\?/i)).toBeInTheDocument();
  expect(screen.getByText(/How long does a Beam live\?/i)).toBeInTheDocument();

  expect(document.body).not.toHaveTextContent(
    /automatically garbage-collected/
  );

  await user.click(
    screen.getByRole('button', { name: /How long does a Beam live\?/i })
  );

  expect(document.body).toHaveTextContent(/automatically garbage-collected/);
});
