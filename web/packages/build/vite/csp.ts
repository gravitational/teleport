import type { Plugin } from 'vite';

export function cspPlugin(csp: string): Plugin {
  return {
    name: 'teleport-connect-html-plugin',
    transformIndexHtml(html) {
      return {
        html,
        tags: [
          {
            tag: 'meta',
            attrs: {
              'http-equiv': 'Content-Security-Policy',
              content: csp,
            },
          },
        ],
      };
    },
  };
}
