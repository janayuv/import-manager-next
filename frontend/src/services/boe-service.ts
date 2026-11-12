import * as BoeBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/boe/service.js';
import * as BoeEntryBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/boe_entry/service.js';
import * as BoeSummaryBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/boe_summary/service.js';
import type {
  BOEAttachment,
  BOECalcLineResult,
  BOECalcParams,
  BOECalculationContext,
  BOECalculationItemInput,
  BOECalculationResult,
  BOECalculationSummary,
  BOEDetail,
  BOEDetailInput,
  BOEOverrideImportResult,
  CSVImportResult,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  BOEAttachment,
  BOECalcLineResult,
  BOECalcParams,
  BOECalculationContext,
  BOECalculationItemInput,
  BOECalculationResult,
  BOECalculationSummary,
  BOEDetail,
  BOEDetailInput,
  BOEOverrideImportResult,
  CSVImportResult,
};

export const boeService = {
  list(search = ''): Promise<BOEDetail[]> {
    return BoeBindings.List(search).then((r) => r ?? []);
  },
  getById(id: string): Promise<BOEDetail> {
    return BoeBindings.GetByID(id);
  },
  create(input: BOEDetailInput): Promise<BOEDetail> {
    return BoeBindings.Create(input);
  },
  update(id: string, input: BOEDetailInput): Promise<BOEDetail> {
    return BoeBindings.Update(id, input);
  },
  delete(id: string): Promise<void> {
    return BoeBindings.Delete(id);
  },
  downloadTemplateCsv(): Promise<string> {
    return BoeBindings.DownloadTemplateCSV();
  },
  importCsv(content: string): Promise<CSVImportResult> {
    return BoeBindings.ImportCSV(content);
  },
  listAttachments(calcId: string): Promise<BOEAttachment[]> {
    return BoeBindings.ListAttachments(calcId).then((r) => r ?? []);
  },
  deleteAttachment(id: string): Promise<void> {
    return BoeBindings.DeleteAttachment(id);
  },
};

export const boeEntryService = {
  loadShipmentContext(shipmentId: string): Promise<BOECalculationContext> {
    return BoeEntryBindings.LoadShipmentContext(shipmentId);
  },
  previewCalculation(params: BOECalcParams): Promise<BOECalculationResult> {
    return BoeEntryBindings.PreviewCalculation(params);
  },
  parseOverrideCsv(csvContent: string, currentItems: BOECalculationItemInput[]): Promise<BOEOverrideImportResult> {
    return BoeEntryBindings.ParseOverrideCSV(csvContent, currentItems);
  },
  saveCalculation(params: BOECalcParams): Promise<BOECalculationSummary> {
    return BoeEntryBindings.SaveCalculation(params);
  },
};

export const boeSummaryService = {
  list(search = '', status = ''): Promise<BOECalculationSummary[]> {
    return BoeSummaryBindings.List(search, status).then((r) => r ?? []);
  },
  updateStatus(calcId: string, status: string): Promise<void> {
    return BoeSummaryBindings.UpdateStatus(calcId, status);
  },
};
