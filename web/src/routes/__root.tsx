import type { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { Layout } from '../shared/ui/Layout';

interface RouterContext {
  queryClient: QueryClient;
}

function RootErrorComponent({ error }: Readonly<{ error: unknown }>) {
  return (
    <div role="alert" className="alert alert-error">
      <span>{error instanceof Error ? error.message : 'Something went wrong.'}</span>
    </div>
  );
}

function NotFound() {
  return (
    <div className="hero py-24">
      <div className="hero-content text-center">
        <div>
          <h1 className="text-3xl font-bold">Page not found</h1>
          <a href="/" className="btn btn-primary mt-6">
            Back to the calendar
          </a>
        </div>
      </div>
    </div>
  );
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <Layout>
      <Outlet />
    </Layout>
  ),
  errorComponent: RootErrorComponent,
  notFoundComponent: NotFound,
});
