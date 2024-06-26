package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	errpkg "github.com/genku-m/upsider-cording-test/invoice/errors"
	"github.com/genku-m/upsider-cording-test/models"
)

type InvoiceStatus string

const (
	InvoiceStatusUnprocessed InvoiceStatus = "unprocessed"
	InvoiceStatusProcessing  InvoiceStatus = "processing"
	InvoiceStatusPaied       InvoiceStatus = "paied"
	InvoiceStatusError       InvoiceStatus = "error"
)

type InvoiceRepository struct {
	db *sql.DB
}

func NewInvoiceRepository(db *sql.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

type Invoice struct {
	ID                uint32    `db:"id"`
	GUID              string    `db:"guid"`
	CompanyID         uint32    `db:"company_id"`
	CustomerID        uint32    `db:"customer_id"`
	PublishDate       time.Time `db:"publish_date"`
	Payment           uint64    `db:"payment"`
	CommissionTax     uint64    `db:"commission"`
	CommissionTaxRate float64   `db:"commission_tax"`
	ConsumptionTax    uint64    `db:"consumption_tax"`
	TaxRate           float64   `db:"tax_rate"`
	BillingAmount     uint64    `db:"billing_amount"`
	PaymentDate       time.Time `db:"payment_date"`
	Status            string    `db:"status"`
}

type InvoiceWithCustomerGUID struct {
	Invoice      `db:"invoice"`
	CustomerGUID string `db:"company.guid"`
}

func (r *InvoiceRepository) Create(ctx context.Context, invoice *models.Invoice) error {
	var customerID, companyID uint32
	var companyGUID string
	err := r.db.QueryRowContext(ctx, "SELECT id, company_id FROM customer WHERE guid=?", invoice.CustomerGUID).Scan(&companyID, &customerID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return errpkg.NewNotFoundError(fmt.Errorf("customer not found: %v err: %v", invoice.CustomerGUID, err.Error()))
		default:
			return errpkg.NewInternalError(err)
		}
	}

	err = r.db.QueryRowContext(ctx, "SELECT guid FROM company WHERE id=?", companyID).Scan(&companyGUID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return errpkg.NewNotFoundError(fmt.Errorf("customer not found: %v err: %v", invoice.CustomerGUID, err.Error()))
		default:
			return errpkg.NewInternalError(err)
		}
	}

	if companyGUID != invoice.CompanyGUID {
		return errpkg.NewInvalidArgumentError(fmt.Errorf("company guid is not match: %v", invoice.CompanyGUID))
	}

	_, err = r.db.ExecContext(ctx, `INSERT INTO invoice (
		guid,
		company_id,
		customer_id,
		publish_date,
		payment,
		commission_tax,
		commission_tax_rate,
		consumption_tax,
		tax_rate,
		billing_amount,
		payment_date,
		status
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		invoice.GUID,
		companyID,
		customerID,
		invoice.PublishDate,
		invoice.Payment,
		invoice.CommissionTax,
		invoice.CommissionTaxRate,
		invoice.ConsumptionTax,
		invoice.TaxRate,
		invoice.BillingAmount,
		invoice.PaymentDate,
		invoice.Status)
	if err != nil {
		return errpkg.NewInternalError(err)
	}
	return nil
}

func (r *InvoiceRepository) List(ctx context.Context, companyGUID string, firstPaymentDate, lastPaymentDate time.Time) ([]*models.Invoice, error) {
	var companyID uint32
	err := r.db.QueryRowContext(ctx, "SELECT id FROM company WHERE guid=?", companyGUID).Scan(&companyID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, errpkg.NewNotFoundError(fmt.Errorf("company not found: %v err: %v", companyGUID, err.Error()))
		default:
			return nil, errpkg.NewInternalError(err)
		}
	}

	var invoices []InvoiceWithCustomerGUID
	rows, err := r.db.QueryContext(ctx, `
	SELECT invoice.*, customer.guid FROM 
	invoice
	JOIN customer ON invoice.customer_id = customer.id
	WHERE invoice.company_id = ? AND invoice.payment_date BETWEEN ? AND ?`, companyID, firstPaymentDate, lastPaymentDate)
	if err != nil {
		return nil, errpkg.NewInternalError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var invoice InvoiceWithCustomerGUID
		if err := rows.Scan(
			&invoice.Invoice.ID,
			&invoice.Invoice.GUID,
			&invoice.Invoice.CompanyID,
			&invoice.Invoice.CustomerID,
			&invoice.Invoice.PublishDate,
			&invoice.Invoice.Payment,
			&invoice.Invoice.CommissionTax,
			&invoice.Invoice.CommissionTaxRate,
			&invoice.Invoice.ConsumptionTax,
			&invoice.Invoice.TaxRate,
			&invoice.Invoice.BillingAmount,
			&invoice.Invoice.PaymentDate,
			&invoice.Invoice.Status,
			&invoice.CustomerGUID,
		); err != nil {
			return nil, errpkg.NewInternalError(err)
		}
		invoices = append(invoices, invoice)
	}

	// If the database is being written to ensure to check for Close
	// errors that may be returned from the driver. The query may
	// encounter an auto-commit error and be forced to rollback changes.
	rerr := rows.Close()
	if rerr != nil {
		return nil, errpkg.NewInternalError(rerr)
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		return nil, errpkg.NewInternalError(err)
	}

	invoiceModels := make([]*models.Invoice, 0, len(invoices))
	for _, invoice := range invoices {
		status, err := toModelsStatus(invoice.Status)
		if err != nil {
			return nil, errpkg.NewInternalError(err)
		}

		invoiceModels = append(invoiceModels, &models.Invoice{
			GUID:              invoice.GUID,
			CompanyGUID:       companyGUID,
			CustomerGUID:      invoice.CustomerGUID,
			PublishDate:       invoice.PublishDate,
			Payment:           invoice.Payment,
			CommissionTax:     invoice.CommissionTax,
			CommissionTaxRate: invoice.CommissionTaxRate,
			ConsumptionTax:    invoice.ConsumptionTax,
			TaxRate:           invoice.TaxRate,
			BillingAmount:     invoice.BillingAmount,
			PaymentDate:       invoice.PaymentDate,
			Status:            status,
		})
	}

	return invoiceModels, nil
}

func toModelsStatus(status string) (models.InvoiceStatus, error) {
	switch status {
	case string(InvoiceStatusUnprocessed):
		return models.InvoiceStatusUnprocessed, nil
	case string(InvoiceStatusProcessing):
		return models.InvoiceStatusProcessing, nil
	case string(InvoiceStatusPaied):
		return models.InvoiceStatusPaied, nil
	case string(InvoiceStatusError):
		return models.InvoiceStatusError, nil
	default:
		return "", fmt.Errorf("unknown status: %v", status)
	}
}
