import { createClient, type Client } from '@connectrpc/connect';
import { DividendsService as DividendsServicePB } from '@bvb-dividends/gen/bvb/dividends/v1/dividends_pb';
import type { Dividend, ListDividendsResponse } from '@bvb-dividends/gen/bvb/dividends/v1/dividends_pb';
import { transport } from '../../../shared/api/transport';

export type { Dividend, ListDividendsResponse };

/** Options accepted by the dividend listing, mirroring the API's parameters. */
export interface ListDividendsOptions {
  /** A company symbol, or undefined to list across every company. */
  company?: string;
  /** An AIP-160 filter expression. */
  filter?: string;
  /** An AIP-132 ordering. */
  orderBy?: string;
  pageSize?: number;
  pageToken?: string;
}

/**
 * Reads dividends from the API.
 *
 * The client is generated from the same protos the service is built from, so a
 * field that changes shape is a compile error here rather than a value that
 * quietly arrives as undefined.
 */
export class DividendsService {
  private readonly client: Client<typeof DividendsServicePB>;

  constructor() {
    this.client = createClient(DividendsServicePB, transport);
  }

  /** Lists dividends, using the AIP-159 wildcard when no company is named. */
  public listDividends(options: ListDividendsOptions): Promise<ListDividendsResponse> {
    return this.client.listDividends({
      parent: `companies/${options.company ?? '-'}`,
      filter: options.filter,
      orderBy: options.orderBy,
      pageSize: options.pageSize,
      pageToken: options.pageToken,
    });
  }
}

export const dividendsService = new DividendsService();
