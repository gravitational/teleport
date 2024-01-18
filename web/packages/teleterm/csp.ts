export function getConnectCsp(development: boolean) {
  // feedbackAddress needs to be kept in sync with the same property in staticConfig.ts.
  const feedbackAddress = development
    ? 'https://kcwm2is93l.execute-api.us-west-2.amazonaws.com/prod'
    : 'https://usage.teleport.dev';

  let csp = `
default-src 'self';
connect-src 'self' ${feedbackAddress};
style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:;
object-src 'none';
font-src 'self' data:;
`
    .replaceAll('\n', ' ')
    .trim();

  if (development) {
    // Required to make source maps work in dev mode.
    csp += " script-src 'self' 'unsafe-eval' 'unsafe-inline';";
  }

  return csp;
}
