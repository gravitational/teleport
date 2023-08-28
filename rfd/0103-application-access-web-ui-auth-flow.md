---
authors: Ryan Clark (ryan.clark@goteleport.com), Lisa Kim (lisa@goteleport.com)
state: implemented
---

# RFD 103 - Application Access Web UI Auth Flow

## What

This is an overview of the flow used for setting the relevant cookies when a user logs in to an
application through Teleport Web UI.

## Why

To document and understand the HTTP application access authn flow for future reference.

## Details

When a user wants to login to an application, a new application session is created in
the auth service. This application session has two values (a name and a bearer token) that need to be provided
as cookies when the user navigates to the application's URL.

The flow to be able to set the needed cookies from the application session across different domains is
as follows (using debug dumper app as example):

```mermaid
sequenceDiagram
    participant Browser
    participant Proxy
    participant Target App
    participant Auth Service
    alt Loading target app outside of web UI
    Note left of Browser: User copy pastes target app URL into browser <br/>https://dumper.localhost:3080
    Browser->>Proxy: Redirect to web launcher, preserving the requested path and query params <br/>In subsequent requests both the path and query gets preserved as path<br/>REDIRECT /web/launch/dumper.localhost?path=<requested-path>&query=<requested-query>
    Note left of Browser: Redirected to app launcher
    else Loading target app within the web UI (using the web app launcher)
    Note left of Browser: At web UI, user clicks on `launch` button for the target app
    Note left of Browser: Web UI routes to app launcher <br/>/web/launch/dumper.localhost/cluster-name/dumper.localhost<br/>route format: /web/launch/:fqdn/:clusterId?/:publicAddr?
    end
    Note right of Browser: App launcher navigates to target app <br/>https://dumper.localhost:3080/x-teleport-auth
    Browser->>Target App: Navigating to target app start's the auth exchange <br/>GET /dumper.localhost/x-teleport-auth
    Target App->>Browser: Create state token, set the token value in a cookie, and as a query param in the redirect URL back to web launcher<br/>REDIRECT /web/launch/dumper.localhost/cluster-name/dumper.localhost?state=<token>
    Note left of Browser: Redirected back to app launcher
    Browser->>Proxy: Create App Session <br>POST /v1/webapi/sessions/app with body:<br>{fqdn: dumper.localhost, plubic_addr: dumper.localhost, cluster_name: cluster-name}
    Proxy->>Auth Service: Create App Session grpc.CreateAppSession
    Auth Service->>Proxy: Returns two cookie values (app session & subject)
    Proxy->>Browser: Returns two cookie values (app session & subject)
    Note right of Browser: App launcher navigates back to target app <br>https://dumper.localhost:3080/x-teleport-auth?state=<token>&subject=<subject-cookie-value>#35;value=<session-cookie-value>
    Browser->>Target App: Continue auth exchange <br>GET /dumper.localhost:3080/x-teleport-auth?state=<token>&subject=<subject-cookie-value>#35;value=<session-cookie-value>
    Target App->>Browser: After checking that a "state" query param exists, serve the app redirection HTML <br>(Just a blank page with inline JS that contains logic to complete auth exchange and redirect to target app path)
    Note right of Browser: Target app loads the redirection HTML
    Browser->>Target App: Complete auth exchange <br>POST /dumper.localhost:3080/x-teleport-auth with body:<br>{state_value: <token>, cookie_value: <session-cookie_value>, subject_cookie: <subject-cookie-value>}
    Target App->>Browser: After verifying the state token matches with the cookie sent automatically by browser and verifying app session,<br>sets the cookies (session cookie and subject cookie) for the target app
    Browser-->Browser: User is authenticated<br>Redirect to the originally requested path https://dumper.localhost:3080 <br/>(In this case it was just the root path)

```

### CSRF Protection

We use the double submit cookie technique to protect against CSRF for the endpoint `POST /target-app/x-teleport-auth` which grants the cookies that is required by the target app (session cookie and subject cookie).

When initiating auth exchange, the backend will create a crypto safe random token and send back this token value as part of a query param called `state` and as a cookie (set on the target app domain).

Call to `POST /target-app/x-teleport-auth` will check that the `state` query param matches with the value of the cookie sent automatically by the browser.
