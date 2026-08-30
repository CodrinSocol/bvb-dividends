import { createClient, type Client } from '@connectrpc/connect';
import { CompaniesService as CompaniesServicePB } from '@bvb-dividends/gen/bvb/dividends/v1/companies_pb';
import type { Company, ListCompaniesResponse } from '@bvb-dividends/gen/bvb/dividends/v1/companies_pb';
import { transport } from '../../../shared/api/transport';

export type { Company, ListCompaniesResponse };

/** Reads companies from the API. */
export class CompaniesService {
  private readonly client: Client<typeof CompaniesServicePB>;

  constructor() {
    this.client = createClient(CompaniesServicePB, transport);
  }

  /** Gets one company by ticker symbol. */
  public getCompany(symbol: string): Promise<Company> {
    return this.client.getCompany({ name: `companies/${symbol}` });
  }

  /** Lists the companies known to have announced a dividend. */
  public listCompanies(pageSize = 200): Promise<ListCompaniesResponse> {
    return this.client.listCompanies({ pageSize, orderBy: 'symbol' });
  }
}

export const companiesService = new CompaniesService();
