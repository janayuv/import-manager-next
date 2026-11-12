import * as ShipmentBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/shipment/service.js';
import * as InvoiceBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/invoice/service.js';
import * as WizardBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/invoice_wizard/service.js';
import type {
  CSVImportResult,
  InvoiceDetail,
  InvoiceLineInput,
  InvoiceLineItem,
  InvoiceSummary,
  ParsedPasteLine,
  SaveInvoiceRequest,
  Shipment,
  ShipmentDetail,
  ShipmentInput,
  ShipmentPage,
  ShipmentPageRequest,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  CSVImportResult,
  InvoiceDetail,
  InvoiceLineInput,
  InvoiceLineItem,
  InvoiceSummary,
  ParsedPasteLine,
  SaveInvoiceRequest,
  Shipment,
  ShipmentDetail,
  ShipmentInput,
  ShipmentPage,
  ShipmentPageRequest,
};

export const shipmentService = {
  list(req: ShipmentPageRequest): Promise<ShipmentPage> {
    return ShipmentBindings.List(req);
  },
  getById(id: string): Promise<Shipment> {
    return ShipmentBindings.GetByID(id);
  },
  getDetail(id: string): Promise<ShipmentDetail> {
    return ShipmentBindings.GetDetail(id);
  },
  create(input: ShipmentInput): Promise<Shipment> {
    return ShipmentBindings.Create(input);
  },
  update(id: string, input: ShipmentInput): Promise<Shipment> {
    return ShipmentBindings.Update(id, input);
  },
  freeze(id: string): Promise<Shipment> {
    return ShipmentBindings.Freeze(id);
  },
  unfreeze(id: string): Promise<Shipment> {
    return ShipmentBindings.Unfreeze(id);
  },
  delete(id: string): Promise<void> {
    return ShipmentBindings.Delete(id);
  },
  checkReadyForDelivery(referenceDate = ''): Promise<number> {
    return ShipmentBindings.CheckReadyForDelivery(referenceDate);
  },
  downloadTemplateCsv(): Promise<string> {
    return ShipmentBindings.DownloadTemplateCSV();
  },
  importCsv(fileName: string, content: string): Promise<CSVImportResult> {
    return ShipmentBindings.ImportCSV(fileName, content);
  },
};

export const invoiceService = {
  list(search = '', status = ''): Promise<InvoiceSummary[]> {
    return InvoiceBindings.List(search, status).then((rows) => rows ?? []);
  },
  getDetail(id: string): Promise<InvoiceDetail> {
    return InvoiceBindings.GetDetail(id);
  },
  delete(id: string): Promise<void> {
    return InvoiceBindings.Delete(id);
  },
  listActiveShipments(): Promise<Shipment[]> {
    return InvoiceBindings.ListActiveShipmentsForInvoicing().then((rows) => rows ?? []);
  },
};

export const invoiceWizardService = {
  parseMultiLinePaste(rawText: string): Promise<ParsedPasteLine[]> {
    return WizardBindings.ParseMultiLinePaste(rawText).then((rows) => rows ?? []);
  },
  saveInvoice(req: SaveInvoiceRequest): Promise<InvoiceDetail> {
    return WizardBindings.SaveInvoice(req);
  },
};
