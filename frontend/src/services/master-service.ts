import * as OptionsBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/options/service.js';
import * as SupplierBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/supplier/service.js';
import * as ItemBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/item_master/service.js';
import type {
  CSVImportResult,
  CSVImportRowError,
  Item,
  ItemInput,
  ItemPage,
  ItemPageRequest,
  LookupOption,
  Supplier,
  SupplierHistoryItem,
  SupplierHistorySummary,
  SupplierInput,
  SupplierPage,
  SupplierPageRequest,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  CSVImportResult,
  CSVImportRowError,
  Item,
  ItemInput,
  ItemPage,
  ItemPageRequest,
  LookupOption,
  Supplier,
  SupplierHistoryItem,
  SupplierHistorySummary,
  SupplierInput,
  SupplierPage,
  SupplierPageRequest,
};

export const optionsService = {
  list(table: string): Promise<LookupOption[]> {
    return OptionsBindings.ListOptions(table).then((rows) => rows ?? []);
  },
  listAll(): Promise<Record<string, LookupOption[]>> {
    return OptionsBindings.ListAllOptions().then((map) => {
      const out: Record<string, LookupOption[]> = {};
      for (const [k, v] of Object.entries(map ?? {})) {
        out[k] = v ?? [];
      }
      return out;
    });
  },
  upsert(table: string, value: string, label: string): Promise<LookupOption> {
    return OptionsBindings.UpsertOption(table, value, label);
  },
  delete(table: string, value: string): Promise<void> {
    return OptionsBindings.DeleteOption(table, value);
  },
};

export const supplierService = {
  list(req: SupplierPageRequest): Promise<SupplierPage> {
    return SupplierBindings.List(req);
  },
  getById(id: string): Promise<Supplier> {
    return SupplierBindings.GetByID(id);
  },
  checkDuplicate(name: string, email: string, excludeId = ''): Promise<Supplier | null> {
    return SupplierBindings.CheckDuplicate(name, email, excludeId);
  },
  getHistory(supplierId: string): Promise<SupplierHistorySummary> {
    return SupplierBindings.GetHistory(supplierId);
  },
  create(input: SupplierInput): Promise<Supplier> {
    return SupplierBindings.Create(input);
  },
  update(id: string, input: SupplierInput): Promise<Supplier> {
    return SupplierBindings.Update(id, input);
  },
  delete(id: string): Promise<void> {
    return SupplierBindings.Delete(id);
  },
  exportCsv(): Promise<string> {
    return SupplierBindings.ExportCSV();
  },
  downloadTemplateCsv(): Promise<string> {
    return SupplierBindings.DownloadTemplateCSV();
  },
  importCsv(content: string): Promise<CSVImportResult> {
    return SupplierBindings.ImportCSV(content);
  },
};

export const itemService = {
  list(req: ItemPageRequest): Promise<ItemPage> {
    return ItemBindings.List(req);
  },
  getById(id: string): Promise<Item> {
    return ItemBindings.GetByID(id);
  },
  checkPartNumber(partNumber: string, excludeId = ''): Promise<Item | null> {
    return ItemBindings.CheckPartNumber(partNumber, excludeId);
  },
  create(input: ItemInput): Promise<Item> {
    return ItemBindings.Create(input);
  },
  update(id: string, input: ItemInput): Promise<Item> {
    return ItemBindings.Update(id, input);
  },
  delete(id: string): Promise<void> {
    return ItemBindings.Delete(id);
  },
  exportCsv(): Promise<string> {
    return ItemBindings.ExportCSV();
  },
  downloadTemplateCsv(): Promise<string> {
    return ItemBindings.DownloadTemplateCSV();
  },
  importCsv(content: string): Promise<CSVImportResult> {
    return ItemBindings.ImportCSV(content);
  },
};
