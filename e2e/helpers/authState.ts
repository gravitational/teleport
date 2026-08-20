import type { StorageState } from './login';

// Sessions are minted on first use rather than up front for every declared user. Bunching them together drew
// the whole run's worth of tokens from the proxy's rate limiter inside a few hundred milliseconds, and the auth
// side's challenge limiter buckets every login together no matter which address the request claims.
//
// The promise is cached rather than the result, so concurrent callers share one login. Module scope survives
// across tests within a Playwright worker, which is what makes the cache worth having.
const states = new Map<string, Promise<StorageState>>();

// authStateFor returns a logged-in storage state for the user, reusing the one already minted in this process.
export function authStateFor(username: string) {
  let state = states.get(username);
  if (state) {
    return state;
  }

  state = login(username);
  states.set(username, state);

  return state;
}

async function login(username: string) {
  // Imported lazily because helpers/env throws on a missing E2E_USERS_FILE, and runs against an existing
  // cluster never reach this path.
  const [{ startUrl, users }, { directLogin }] = await Promise.all([
    import('./env'),
    import('./login'),
  ]);

  const creds = users[username];
  if (!creds) {
    throw new Error(`no credentials found for user "${username}"`);
  }

  return directLogin(startUrl, username, creds.password, creds.clientIp);
}
