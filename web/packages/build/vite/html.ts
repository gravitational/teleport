/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { readFileSync } from 'fs';
import type * as http from 'http';
import * as https from 'https';
import { isIP } from 'net';
import { resolve } from 'path';

import type { Plugin } from 'vite';

export function htmlPlugin(target: string): Plugin {
  return {
    name: 'teleport-html-plugin',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.url === '/') {
          res.writeHead(302, { Location: '/web' });
          res.end();
          return;
        }
        next();
      });

      return () => {
        server.middlewares.use(async (req, res, next) => {
          if (req.url !== '/index.html') {
            next();
            return;
          }

          try {
            const { body, headers } = await fetchIndexHtml(req.headers, target);

            if (cachedTemplate === null) {
              cachedTemplate = readFileSync(indexHtmlPath, 'utf-8');
            }

            const transformed = await server.transformIndexHtml(
              req.originalUrl,
              cachedTemplate
            );

            const bearerToken = body.match(BEARER_META_RE)?.[1] ?? '';
            const csrfToken = body.match(CSRF_META_RE)?.[1] ?? '';
            const html = transformed
              .replace(
                BEARER_META_RE,
                `<meta name="grv_bearer_token" content="${bearerToken}">`
              )
              .replace(
                CSRF_META_RE,
                `<meta name="grv_csrf_token" content="${csrfToken}">`
              );

            if (headers['set-cookie']) {
              res.setHeader('set-cookie', headers['set-cookie']);
            }

            res.writeHead(200, { 'Content-Type': 'text/html' });
            res.end(html);
          } catch (err) {
            server.ssrFixStacktrace(err);
            next(err);
          }
        });
      };
    },
  };
}

export function transformPlugin(): Plugin {
  return {
    name: 'teleport-transform-html-plugin',
    transformIndexHtml(html) {
      return {
        html,
        tags: [{ tag: 'script', attrs: { src: '/web/config.js' } }],
      };
    },
  };
}

// Headers that shouldn't be forwarded to the upstream request.
const FORBIDDEN_HEADERS = new Set([
  'connection',
  'proxy-connection',
  'keep-alive',
  'transfer-encoding',
  'upgrade',
  'host',
]);

// Captures the content of `<meta name="grv_bearer_token" content="…">` with
// either quote style and arbitrary attribute order.
const BEARER_META_RE =
  /<meta\s+[^>]*name=["']grv_bearer_token["'][^>]*content=["']([^"']*)["'][^>]*>/i;

// Captures the content of `<meta name="grv_csrf_token" content="…">` with
// either quote style and arbitrary attribute order.
const CSRF_META_RE =
  /<meta\s+[^>]*name=["']grv_csrf_token["'][^>]*content=["']([^"']*)["'][^>]*>/i;

const indexHtmlPath = resolve(process.cwd(), 'index.html');
// index.html can't change during a dev session (vite restarts on config changes), so
// read it once.
let cachedTemplate: string | null = null;

// Cached HTTPS agent for connection reuse.
let cachedAgent: https.Agent | null = null;

function getAgent(target: string): https.Agent {
  if (cachedAgent) {
    return cachedAgent;
  }

  const { hostname } = new URL(`https://${target}`);

  cachedAgent = new https.Agent({
    rejectUnauthorized: false,
    // SNI must not be an IP literal. Newer Node will silently drop the IP
    // servername, which can leave the connection in a broken state; suppress
    // it explicitly here.
    ...(isIP(hostname) && { servername: '' }),
  });

  return cachedAgent;
}

function fetchIndexHtml(reqHeaders: http.IncomingHttpHeaders, target: string) {
  const headers: http.OutgoingHttpHeaders = {
    host: target,
  };

  for (const [name, value] of Object.entries(reqHeaders)) {
    if (value == null || name.startsWith(':')) {
      continue;
    }

    if (FORBIDDEN_HEADERS.has(name.toLowerCase())) {
      continue;
    }

    headers[name] = value;
  }

  const { hostname, port } = new URL(`https://${target}`);

  return new Promise<{ body: string; headers: http.IncomingHttpHeaders }>(
    (resolve, reject) => {
      const req = https.request(
        {
          hostname,
          port: port || 443,
          path: '/web',
          method: 'GET',
          headers,
          agent: getAgent(target),
        },
        res => {
          let body = '';
          res.setEncoding('utf8');
          res.on('data', chunk => {
            body += chunk;
          });
          res.on('end', () => resolve({ body, headers: res.headers }));
        }
      );

      req.on('error', reject);
      req.end();
    }
  );
}
