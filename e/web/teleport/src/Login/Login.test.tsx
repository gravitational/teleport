import { MemoryRouter } from 'react-router';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import cfg from 'teleport/config';
import auth from 'teleport/services/auth/auth';
import history from 'teleport/services/history';
import session from 'teleport/services/websession';

import { LoginContainer as Login } from './Login';

function LoginTest({ initialURL }: { initialURL?: string }) {
  const initialEntries = initialURL ? [initialURL] : undefined;
  return (
    <MemoryRouter initialEntries={initialEntries}>
      <Login />
    </MemoryRouter>
  );
}

beforeEach(() => {
  jest.restoreAllMocks();
  jest.spyOn(history, 'push').mockImplementation();
  jest.spyOn(history, 'replace').mockImplementation();
  jest.spyOn(history, 'getRedirectParam').mockImplementation(() => '/');
  jest.spyOn(history, 'hasAccessChangedParam').mockImplementation(() => false);
});

test('basic rendering', () => {
  render(<LoginTest />);

  // test rendering of logo and title
  expect(screen.getByRole('img')).toBeInTheDocument();
  expect(screen.getByText(/sign in to teleport/i)).toBeInTheDocument();
});

test('renders Beams branding when beamsUi is enabled', () => {
  jest.spyOn(cfg, 'getBeamsUi').mockReturnValue(true);

  render(<LoginTest />);

  expect(screen.getByText('Sign in to Beams')).toBeInTheDocument();
  expect(screen.queryByText('Sign in to Teleport')).not.toBeInTheDocument();
});

describe.each([
  {
    name: 'unscoped',
    url: '/web/login',
    scope: '',
    ssoURL:
      'http://localhost/v1/webapi/github/login/web?connector_id=github&redirect_url=http%3A%2F%2Flocalhost%2Fweb',
  },
  {
    name: 'scoped',
    url: '/web/login?scope=%2Fdev',
    scope: '/dev',
    ssoURL:
      'http://localhost/v1/webapi/github/login/web?connector_id=github&scope=%2Fdev&redirect_url=http%3A%2F%2Flocalhost%2Fweb',
  },
])('$name', ({ url, scope, ssoURL }) => {
  test('login with redirect', async () => {
    jest.spyOn(auth, 'login').mockResolvedValue({});

    render(<LoginTest initialURL={url} />);

    // fill form
    const user = userEvent.setup();
    await user.type(screen.getByPlaceholderText(/username/i), 'username');
    await user.type(screen.getByPlaceholderText(/password/i), '123');

    // test login and redirect
    await user.click(screen.getByText('Sign In'));
    await waitFor(() => {
      expect(auth.login).toHaveBeenCalledWith(
        'username',
        '123',
        '', // otp
        scope
      );
    });
    expect(history.push).toHaveBeenCalledWith('http://localhost/web', true);
  });

  test('login with SSO', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'otp');
    jest.spyOn(cfg, 'getPrimaryAuthType').mockImplementation(() => 'sso');
    jest.spyOn(cfg, 'getAuthProviders').mockImplementation(() => [
      {
        displayName: 'With GitHub',
        type: 'github',
        name: 'github',
        url: '/v1/webapi/github/login/web?connector_id=:providerName&scope=:scope?&redirect_url=:redirect',
      },
    ]);

    render(<LoginTest initialURL={url} />);

    // test login pathways
    const user = userEvent.setup();
    await user.click(screen.getByText('With GitHub'));
    expect(history.push).toHaveBeenCalledWith(ssoURL, true);
  });
});

describe('test MOTD', () => {
  test('show motd only if motd is set', async () => {
    // default login form
    const { unmount } = render(<LoginTest />);
    expect(screen.getByPlaceholderText(/username/i)).toBeInTheDocument();
    expect(
      screen.queryByText('Welcome to cluster, your activity will be recorded.')
    ).not.toBeInTheDocument();
    unmount();

    // now set motd
    jest
      .spyOn(cfg, 'getMotd')
      .mockImplementation(
        () => 'Welcome to cluster, your activity will be recorded.'
      );

    render(<LoginTest />);

    expect(
      screen.getByText('Welcome to cluster, your activity will be recorded.')
    ).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/username/i)).not.toBeInTheDocument();
  });

  test('show login form after modt acknowledge', async () => {
    jest
      .spyOn(cfg, 'getMotd')
      .mockImplementation(
        () => 'Welcome to cluster, your activity will be recorded.'
      );
    render(<LoginTest />);
    expect(
      screen.getByText('Welcome to cluster, your activity will be recorded.')
    ).toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByText('Acknowledge'));
    expect(screen.getByPlaceholderText(/username/i)).toBeInTheDocument();
  });

  test('skip motd if login initiated from headless auth', async () => {
    jest
      .spyOn(cfg, 'getMotd')
      .mockImplementation(
        () => 'Welcome to cluster, your activity will be recorded.'
      );
    jest
      .spyOn(history, 'getRedirectParam')
      .mockReturnValue(
        'https://teleport.example.com/web/headless/5c5c1f73-ac5c-52ee-bc9e-0353094dcb4a'
      );

    render(<LoginTest />);

    expect(
      screen.queryByText('Welcome to cluster, your activity will be recorded.')
    ).not.toBeInTheDocument();
  });
});

test('redirect to root if session is valid and path is not "/enterprise/saml-idp/sso', () => {
  jest.spyOn(session, 'isValid').mockImplementation(() => true);
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(
      'http://localhost/web/login?redirect_url=http://localhost/web/cluster/localhost/resources'
    );
  render(<LoginTest />);

  expect(history.replace).toHaveBeenCalledWith('/web');
});

test('redirect to SAML path if session is valid and path matches "/enterprise/saml-idp/sso"', () => {
  const samlIdPPath = new URL('http://localhost' + cfg.routes.samlIdpSso);
  jest.spyOn(session, 'isValid').mockImplementation(() => true);
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(samlIdPPath.toString());
  render(<LoginTest />);
  expect(history.push).toHaveBeenCalledWith(samlIdPPath.toString(), true);
});
