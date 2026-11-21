import * as AppBindings from '../../bindings/github.com/janayuv/import-manager-next/appservice.js';
import type { Company, CreateCompanyInput, UpdateCompanyInput } from '@/types/company';

export const companyService = {
  async listCompanies(): Promise<Company[]> {
    const list = await AppBindings.ListCompanies();
    return (list ?? []) as Company[];
  },

  async getActiveCompany(): Promise<Company> {
    const active = await AppBindings.GetActiveCompany();
    return active as Company;
  },

  async createCompany(input: CreateCompanyInput): Promise<Company> {
    const created = await AppBindings.CreateCompany(input as any);
    return created as Company;
  },

  async updateCompany(input: UpdateCompanyInput): Promise<Company> {
    const updated = await AppBindings.UpdateCompany(input as any);
    return updated as Company;
  },

  async switchCompany(companyID: string): Promise<Company> {
    const switched = await AppBindings.SwitchCompany(companyID);
    return switched as Company;
  },

  async deleteCompany(companyID: string): Promise<void> {
    await AppBindings.DeleteCompany(companyID);
  },
};
