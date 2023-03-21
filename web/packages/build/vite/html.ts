/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { readFileSync } from 'fs';
import { get } from 'https';
import { resolve } from 'path';

import { JSDOM } from 'jsdom';

import type { Plugin } from 'vite';
import type { IncomingHttpHeaders } from 'http';

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
