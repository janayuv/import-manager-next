import * as ExpensesBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/expenses/service.js';
import * as ExpenseReportBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/expense_reports/service.js';
import * as ReportBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/reports/service.js';
import type {
  CSVImportResult,
  ExpenseAttachment,
  ExpenseInvoice,
  ExpenseInvoiceCalculation,
  ExpenseInvoiceRequest,
  ExpenseLine,
  ExpenseLineInput,
  ExpenseReportFilter,
  ExpenseReportResult,
  ExpenseType,
  ExpenseTypeInput,
  LandedCostFilter,
  LandedCostReport,
  ServiceProvider,
  ServiceProviderInput,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  CSVImportResult,
  ExpenseAttachment,
  ExpenseInvoice,
  ExpenseInvoiceCalculation,
  ExpenseInvoiceRequest,
  ExpenseLine,
  ExpenseLineInput,
  ExpenseReportFilter,
  ExpenseReportResult,
  ExpenseType,
  ExpenseTypeInput,
  LandedCostFilter,
  LandedCostReport,
  ServiceProvider,
  ServiceProviderInput,
};

export function formatPaiseINR(paise: number): string {
  const rupees = Math.round(paise / 100);
  return rupees.toLocaleString('en-IN', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  });
}

export const expenseService = {
  listServiceProviders(search = ''): Promise<ServiceProvider[]> {
    return ExpensesBindings.ListServiceProviders(search).then((r) => r ?? []);
  },
  createServiceProvider(input: ServiceProviderInput): Promise<ServiceProvider> {
    return ExpensesBindings.CreateServiceProvider(input);
  },
  updateServiceProvider(id: string, input: ServiceProviderInput): Promise<ServiceProvider> {
    return ExpensesBindings.UpdateServiceProvider(id, input);
  },
  deleteServiceProvider(id: string): Promise<void> {
    return ExpensesBindings.DeleteServiceProvider(id);
  },

  listExpenseTypes(activeOnly = false): Promise<ExpenseType[]> {
    return ExpensesBindings.ListExpenseTypes(activeOnly).then((r) => r ?? []);
  },
  createExpenseType(input: ExpenseTypeInput): Promise<ExpenseType> {
    return ExpensesBindings.CreateExpenseType(input);
  },
  updateExpenseType(id: string, input: ExpenseTypeInput): Promise<ExpenseType> {
    return ExpensesBindings.UpdateExpenseType(id, input);
  },
  deleteExpenseType(id: string): Promise<void> {
    return ExpensesBindings.DeleteExpenseType(id);
  },

  previewCalculation(lines: ExpenseLineInput[]): Promise<ExpenseInvoiceCalculation> {
    return ExpensesBindings.PreviewCalculation(lines);
  },
  mergeDuplicateLines(lines: ExpenseLineInput[]): Promise<ExpenseLineInput[]> {
    return ExpensesBindings.MergeDuplicateLinesAction(lines).then((r) => r ?? []);
  },

  listExpenseInvoices(shipmentId = '', search = ''): Promise<ExpenseInvoice[]> {
    return ExpensesBindings.ListExpenseInvoices(shipmentId, search).then((r) => r ?? []);
  },
  getExpenseInvoice(id: string): Promise<ExpenseInvoice> {
    return ExpensesBindings.GetExpenseInvoice(id);
  },
  createExpenseInvoice(req: ExpenseInvoiceRequest): Promise<ExpenseInvoice> {
    return ExpensesBindings.CreateExpenseInvoice(req);
  },
  updateExpenseInvoice(req: ExpenseInvoiceRequest): Promise<ExpenseInvoice> {
    return ExpensesBindings.UpdateExpenseInvoice(req);
  },
  deleteExpenseInvoice(id: string): Promise<void> {
    return ExpensesBindings.DeleteExpenseInvoice(id);
  },

  listAttachments(expenseId: string): Promise<ExpenseAttachment[]> {
    return ExpensesBindings.ListAttachments(expenseId).then((r) => r ?? []);
  },
  uploadAttachment(
    expenseId: string,
    fileName: string,
    fileType: string,
    base64Data: string,
  ): Promise<ExpenseAttachment> {
    return ExpensesBindings.UploadAttachment(expenseId, fileName, fileType, base64Data);
  },
  deleteAttachment(id: string): Promise<void> {
    return ExpensesBindings.DeleteAttachment(id);
  },

  downloadBulkTemplateCsv(): Promise<string> {
    return ExpensesBindings.DownloadBulkTemplateCSV();
  },
  importBulkCsv(shipmentId: string, content: string): Promise<CSVImportResult> {
    return ExpensesBindings.ImportBulkCSV(shipmentId, content);
  },
};

export const expenseReportsService = {
  getReport(filter: ExpenseReportFilter): Promise<ExpenseReportResult> {
    return ExpenseReportBindings.GetExpenseReport(filter);
  },
};

export const reportsService = {
  getLandedCostReport(filter: LandedCostFilter): Promise<LandedCostReport> {
    return ReportBindings.GetLandedCostReport(filter);
  },
  exportLandedCostCsv(filter: LandedCostFilter): Promise<string> {
    return ReportBindings.ExportLandedCostCSV(filter);
  },
};
