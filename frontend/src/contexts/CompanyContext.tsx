import * as React from 'react';
import { toast } from 'sonner';
import { companyService } from '@/services/company-service';
import type { Company, CreateCompanyInput, UpdateCompanyInput } from '@/types/company';

interface CompanyContextValue {
  loading: boolean;
  currentCompany: Company | null;
  companies: Company[];
  switchCompany: (companyID: string) => Promise<void>;
  createCompany: (input: CreateCompanyInput) => Promise<Company>;
  updateCompany: (input: UpdateCompanyInput) => Promise<Company>;
  deleteCompany: (companyID: string) => Promise<void>;
  refreshCompanies: () => Promise<void>;
}

const CompanyContext = React.createContext<CompanyContextValue | null>(null);

export function CompanyProvider({ children }: { children: React.ReactNode }) {
  const [loading, setLoading] = React.useState(true);
  const [switching, setSwitching] = React.useState(false);
  const [currentCompany, setCurrentCompany] = React.useState<Company | null>(null);
  const [companies, setCompanies] = React.useState<Company[]>([]);

  const refreshCompanies = React.useCallback(async () => {
    try {
      const [list, active] = await Promise.all([
        companyService.listCompanies(),
        companyService.getActiveCompany(),
      ]);
      setCompanies(Array.isArray(list) ? list : []);
      setCurrentCompany(active ?? null);
    } catch (err) {
      console.error('Failed to load companies:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refreshCompanies();
  }, [refreshCompanies]);

  const switchCompany = React.useCallback(
    async (companyID: string) => {
      if (currentCompany && currentCompany.id === companyID) return;
      setSwitching(true);
      const loadingToast = toast.loading('Switching company database...');
      try {
        const next = await companyService.switchCompany(companyID);
        setCurrentCompany(next);
        await refreshCompanies();
        toast.dismiss(loadingToast);
        toast.success(`Switched active company to ${next.name} (${next.shortCode})`);
        
        // Fast desktop workspace remount to ensure complete state isolation
        setTimeout(() => {
          window.location.hash = '#/';
          window.location.reload();
        }, 300);
      } catch (err: any) {
        toast.dismiss(loadingToast);
        toast.error(err?.message || 'Failed to switch company');
        setSwitching(false);
      }
    },
    [currentCompany, refreshCompanies],
  );

  const createCompany = React.useCallback(
    async (input: CreateCompanyInput): Promise<Company> => {
      try {
        const created = await companyService.createCompany(input);
        toast.success(`Company '${created.name}' (${created.shortCode}) created successfully`);
        await refreshCompanies();
        if (input.switchImmediately) {
          setCurrentCompany(created);
          setTimeout(() => {
            window.location.hash = '#/';
            window.location.reload();
          }, 300);
        }
        return created;
      } catch (err: any) {
        toast.error(err?.message || 'Failed to create company');
        throw err;
      }
    },
    [refreshCompanies],
  );

  const updateCompany = React.useCallback(
    async (input: UpdateCompanyInput): Promise<Company> => {
      try {
        const updated = await companyService.updateCompany(input);
        toast.success(`Company details for '${updated.name}' updated`);
        await refreshCompanies();
        return updated;
      } catch (err: any) {
        toast.error(err?.message || 'Failed to update company');
        throw err;
      }
    },
    [refreshCompanies],
  );

  const deleteCompany = React.useCallback(
    async (companyID: string): Promise<void> => {
      try {
        await companyService.deleteCompany(companyID);
        toast.success('Company removed successfully');
        await refreshCompanies();
      } catch (err: any) {
        toast.error(err?.message || 'Failed to delete company');
        throw err;
      }
    },
    [refreshCompanies],
  );

  const value = React.useMemo<CompanyContextValue>(
    () => ({
      loading,
      currentCompany,
      companies,
      switchCompany,
      createCompany,
      updateCompany,
      deleteCompany,
      refreshCompanies,
    }),
    [
      loading,
      currentCompany,
      companies,
      switchCompany,
      createCompany,
      updateCompany,
      deleteCompany,
      refreshCompanies,
    ],
  );

  if (switching) {
    return (
      <div className="flex min-h-screen w-full flex-col items-center justify-center bg-background p-6 text-foreground">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 animate-spin rounded-full border-3 border-primary border-t-transparent" />
          <h2 className="text-base font-semibold">Switching Corporate Database...</h2>
          <p className="text-xs text-muted-foreground">Flushing WAL buffers and mounting isolated company storage</p>
        </div>
      </div>
    );
  }

  return <CompanyContext.Provider value={value}>{children}</CompanyContext.Provider>;
}

const defaultCompanyContext: CompanyContextValue = {
  loading: false,
  currentCompany: {
    id: 'comp_default',
    name: 'Primary Company',
    shortCode: 'PRIMARY',
    iecCode: '',
    gstin: '',
    pan: '',
    addressLine1: '',
    addressLine2: '',
    city: '',
    state: '',
    postalCode: '',
    country: 'India',
    phone: '',
    email: '',
    website: '',
    beneficiaryName: '',
    bankName: '',
    accountNumber: '',
    ifscCode: '',
    swiftCode: '',
    bankBranch: '',
    bankAddress: '',
    logoData: '',
    baseCurrency: 'INR',
    financialYearFrom: '04-01',
    financialYearTo: '03-31',
    isDefault: true,
    isActive: true,
  },
  companies: [],
  switchCompany: async () => {},
  createCompany: async () => ({} as any),
  updateCompany: async () => ({} as any),
  deleteCompany: async () => {},
  refreshCompanies: async () => {},
};

export function useCompany(): CompanyContextValue {
  const context = React.useContext(CompanyContext);
  return context ?? defaultCompanyContext;
}
