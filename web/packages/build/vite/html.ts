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
import type { IncomingHttpHeaders } from 'http';
import { get } from 'https';
import { resolve } from 'path';

import { JSDOM } from 'jsdom';
import type { Plugin } from 'vite';

function getHTML(target: string, headers: IncomingHttpHeaders) {
  return new Promise<{ data: string; headers: IncomingHttpHeaders }>(
    (resolve, reject) => {
      headers.host = target;

      get(
        `https://${target}/web`,
        { headers, rejectUnauthorized: false },
        res => {
          let data = '';

          res.on('data', d => {
            data += d.toString();
          });

          res.on('end', () => {
            resolve({ data, headers: res.headers });
          });

          res.on('error', reject);
        }
      ).on('error', reject);
    }
  );
}

function replaceMetaTag(name: string, source: JSDOM, target: JSDOM) {
  const sourceTag = source.window.document.querySelector(
    `meta[name=${name}]`
  ) as HTMLMetaElement;
  const targetTag = target.window.document.querySelector(
    `meta[name=${name}]`
  ) as HTMLMetaElement;

  const value = sourceTag.getAttribute('content');

  targetTag.setAttribute('content', value);
}

export function transformPlugin(): Plugin {
  return {
    name: 'teleport-transform-html-plugin',
    transformIndexHtml(html) {
      return {
        html,
        tags: [
          {
            tag: 'script',
            attrs: {
              src: '/web/config.js',
            },
          },
        ],
      };
    },
  };
}

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
          try {
            const { data, headers } = await getHTML(target, req.headers);

            let template = readFileSync(
              resolve(process.cwd(), 'index.html'),
              'utf-8'
            );

            template = await server.transformIndexHtml(
              req.originalUrl,
              template
            );

            const source = new JSDOM(data);
            const result = new JSDOM(template);

            replaceMetaTag('grv_csrf_token', source, result);
            replaceMetaTag('grv_bearer_token', source, result);

            res.setHeader('set-cookie', headers['set-cookie']);
            res.writeHead(200, { 'Content-Type': 'text/html' });
            res.write(result.serialize());
            res.end();
          } catch (err) {
            server.ssrFixStacktrace(err);
            next(err);
          }
        });
      };
    },
  };
}
