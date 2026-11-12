package invoice

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service manages commercial invoice listing, inspection, active shipment candidates, and soft deletion.
type Service struct {
	repo     *repositories.InvoiceRepository
	shipRepo *repositories.ShipmentRepository
	sec      *security.Service
	audit    *audit.Service
}

// NewService constructs a commercial invoice Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		repo:     repositories.NewInvoiceRepository(db),
		shipRepo: repositories.NewShipmentRepository(db),
		sec:      sec,
		audit:    auditSvc,
	}
}

// List returns invoices matching optional search & status filter.
func (s *Service) List(search, status string) ([]models.InvoiceSummary, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.repo.ListAll(search, status)
}

// GetDetail fetches invoice header metadata and all enriched line items.
func (s *Service) GetDetail(invoiceID string) (models.InvoiceDetail, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.InvoiceDetail{}, err
	}
	return s.repo.GetDetail(invoiceID)
}

// Delete soft-deletes an invoice unless its parent shipment is frozen.
func (s *Service) Delete(invoiceID string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	detail, err := s.repo.GetDetail(invoiceID)
	if err != nil {
		return err
	}
	if detail.Invoice.ShipmentFrozen {
		return fmt.Errorf("cannot delete invoice %s because shipment %s is frozen", invoiceID, detail.Invoice.ShipmentID)
	}
	if err := s.repo.SoftDelete(invoiceID, sess.UserID); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("invoices", invoiceID, "DELETE", sess.UserID, detail, nil, map[string]any{"softDelete": true})
	return nil
}

// ListActiveShipmentsForInvoicing lists non-frozen, non-terminal shipments eligible for invoice entry.
func (s *Service) ListActiveShipmentsForInvoicing() ([]models.Shipment, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	page, err := s.shipRepo.List(models.ShipmentPageRequest{Page: 1, PageSize: 500})
	if err != nil {
		return nil, err
	}
	var out []models.Shipment
	for _, sh := range page.Items {
		if sh.IsFrozen {
			continue
		}
		st := strings.ToLower(strings.TrimSpace(sh.Status))
		if st == "delivered" || st == "cancelled" || st == "closed" || st == "completed" {
			continue
		}
		out = append(out, sh)
	}
	return out, nil
}
