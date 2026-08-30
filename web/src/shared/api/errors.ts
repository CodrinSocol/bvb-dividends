import { Code, ConnectError } from '@connectrpc/connect';

/** Reports whether a failure is the API saying the resource does not exist. */
export function isNotFound(error: unknown): boolean {
  return error instanceof ConnectError && error.code === Code.NotFound;
}

/**
 * Describes a failure in a sentence a reader can act on.
 *
 * A message the API sent is preferred to one invented here: it names the
 * resource or the part of the request that was refused.
 */
export function describeError(error: unknown): string {
  if (error instanceof ConnectError) {
    return error.rawMessage;
  }

  return 'The API could not be reached.';
}
