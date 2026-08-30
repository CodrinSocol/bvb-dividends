import { createConnectTransport } from '@connectrpc/connect-web';

/**
 * How the browser reaches the API.
 *
 * The path is relative, and the same in development and in production: the
 * dev server proxies /api, and a deployed build is served by the binary that
 * serves the API, from the same origin. Nothing here needs configuring.
 */
export const transport = createConnectTransport({
  baseUrl: '/api',
});
