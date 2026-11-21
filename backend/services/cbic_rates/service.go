package cbic_rates

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service manages statutory CBIC customs exchange rate notifications under RBAC and audit control.
type Service struct {
	db    *sql.DB
	sec   *security.Service
	audit *audit.Service
}

// NewService constructs a new CBIC exchange rate Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:    db,
		sec:   sec,
		audit: auditSvc,
	}
}

// SetDB rebinds the service to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
}

// List returns CBIC exchange rate entries, optionally filtered by currency code.
func (s *Service) List(currency string) ([]models.CBICExchangeRate, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}

	q := `
		SELECT id, currency_code, exchange_rate, effective_from, effective_to,
		       notification_number, COALESCE(source_document, ''), verification_status,
		       COALESCE(notes, ''), created_at, created_by
		FROM cbic_exchange_rates`
	var args []any
	curr := strings.ToUpper(strings.TrimSpace(currency))
	if curr != "" && curr != "ALL" {
		q += ` WHERE currency_code = ?`
		args = append(args, curr)
	}
	q += ` ORDER BY effective_from DESC, currency_code ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list cbic exchange rates: %w", err)
	}
	defer rows.Close()

	var out []models.CBICExchangeRate
	for rows.Next() {
		var r models.CBICExchangeRate
		var effTo sql.NullString
		if err := rows.Scan(
			&r.ID, &r.CurrencyCode, &r.ExchangeRate, &r.EffectiveFrom, &effTo,
			&r.NotificationNumber, &r.SourceDocument, &r.VerificationStatus,
			&r.Notes, &r.CreatedAt, &r.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan cbic rate: %w", err)
		}
		if effTo.Valid {
			v := effTo.String
			r.EffectiveTo = &v
		}
		out = append(out, r)
	}
	if out == nil {
		out = []models.CBICExchangeRate{}
	}
	return out, rows.Err()
}

// GetEffectiveRate finds the active statutory CBIC rate for currency on a given reference date (defaulting to today).
func (s *Service) GetEffectiveRate(currency string, targetDate string) (models.CBICExchangeRate, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.CBICExchangeRate{}, err
	}

	curr := strings.ToUpper(strings.TrimSpace(currency))
	if curr == "" {
		return models.CBICExchangeRate{}, errors.New("currencyCode is required")
	}
	date := strings.TrimSpace(targetDate)
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	row := s.db.QueryRow(`
		SELECT id, currency_code, exchange_rate, effective_from, effective_to,
		       notification_number, COALESCE(source_document, ''), verification_status,
		       COALESCE(notes, ''), created_at, created_by
		FROM cbic_exchange_rates
		WHERE currency_code = ?
		  AND effective_from <= ?
		  AND (effective_to IS NULL OR effective_to >= ?)
		  AND verification_status = 'Verified'
		ORDER BY effective_from DESC, id DESC
		LIMIT 1`, curr, date, date)

	var r models.CBICExchangeRate
	var effTo sql.NullString
	if err := row.Scan(
		&r.ID, &r.CurrencyCode, &r.ExchangeRate, &r.EffectiveFrom, &effTo,
		&r.NotificationNumber, &r.SourceDocument, &r.VerificationStatus,
		&r.Notes, &r.CreatedAt, &r.CreatedBy,
	); errors.Is(err, sql.ErrNoRows) {
		return models.CBICExchangeRate{}, fmt.Errorf("no verified CBIC statutory exchange rate found for %s on %s", curr, date)
	} else if err != nil {
		return models.CBICExchangeRate{}, fmt.Errorf("query effective cbic rate: %w", err)
	}
	if effTo.Valid {
		v := effTo.String
		r.EffectiveTo = &v
	}
	return r, nil
}

// Create inserts a new verified/pending CBIC customs exchange rate notification.
func (s *Service) Create(in models.CBICExchangeRateInput) (models.CBICExchangeRate, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.CBICExchangeRate{}, err
	}

	curr := strings.ToUpper(strings.TrimSpace(in.CurrencyCode))
	if curr == "" {
		return models.CBICExchangeRate{}, errors.New("currencyCode is required")
	}
	if in.ExchangeRate <= 0 {
		return models.CBICExchangeRate{}, errors.New("exchangeRate must be greater than 0")
	}
	from := strings.TrimSpace(in.EffectiveFrom)
	if from == "" {
		return models.CBICExchangeRate{}, errors.New("effectiveFrom date is required")
	}
	notif := strings.TrimSpace(in.NotificationNumber)
	if notif == "" {
		return models.CBICExchangeRate{}, errors.New("notificationNumber is required")
	}
	status := strings.TrimSpace(in.VerificationStatus)
	if strings.EqualFold(status, "verified") || status == "" {
		status = "Verified"
	} else if strings.EqualFold(status, "pending") {
		status = "Pending"
	} else {
		return models.CBICExchangeRate{}, errors.New("verificationStatus must be 'Verified' or 'Pending'")
	}

	var effTo *string
	if in.EffectiveTo != nil && strings.TrimSpace(*in.EffectiveTo) != "" {
		cleaned := strings.TrimSpace(*in.EffectiveTo)
		effTo = &cleaned
	}

	var existsCount int
	_ = s.db.QueryRow(`
		SELECT COUNT(*) FROM cbic_exchange_rates
		WHERE currency_code = ? AND notification_number = ?`, curr, notif).Scan(&existsCount)
	if existsCount > 0 {
		return models.CBICExchangeRate{}, fmt.Errorf("CBIC exchange rate for %s under notification %q already exists", curr, notif)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		INSERT INTO cbic_exchange_rates (
			currency_code, exchange_rate, effective_from, effective_to,
			notification_number, source_document, verification_status, notes, created_at, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		curr, in.ExchangeRate, from, effTo,
		notif, nullIfEmpty(in.SourceDocument), status, nullIfEmpty(in.Notes), now, sess.UserID,
	)
	if err != nil {
		return models.CBICExchangeRate{}, fmt.Errorf("insert cbic exchange rate: %w", err)
	}

	id, _ := res.LastInsertId()
	created := models.CBICExchangeRate{
		ID:                 id,
		CurrencyCode:       curr,
		ExchangeRate:       in.ExchangeRate,
		EffectiveFrom:      from,
		EffectiveTo:        effTo,
		NotificationNumber: notif,
		SourceDocument:     in.SourceDocument,
		VerificationStatus: status,
		Notes:              in.Notes,
		CreatedAt:          now,
		CreatedBy:          sess.UserID,
	}

	_ = s.audit.RecordTableAudit("cbic_exchange_rates", fmt.Sprintf("%d", id), "CREATE", sess.UserID, nil, created, nil)
	return created, nil
}

// Delete removes a rate entry.
func (s *Service) Delete(id int64) error {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return err
	}

	var prev models.CBICExchangeRate
	var effTo sql.NullString
	err = s.db.QueryRow(`
		SELECT id, currency_code, exchange_rate, effective_from, effective_to, notification_number, created_by
		FROM cbic_exchange_rates WHERE id = ?`, id).Scan(
		&prev.ID, &prev.CurrencyCode, &prev.ExchangeRate, &prev.EffectiveFrom, &effTo, &prev.NotificationNumber, &prev.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("rate %d not found: %w", id, err)
	}
	if effTo.Valid {
		v := effTo.String
		prev.EffectiveTo = &v
	}

	if _, err := s.db.Exec(`DELETE FROM cbic_exchange_rates WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete rate %d: %w", id, err)
	}

	_ = s.audit.RecordTableAudit("cbic_exchange_rates", fmt.Sprintf("%d", id), "DELETE", sess.UserID, prev, nil, nil)
	return nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}
