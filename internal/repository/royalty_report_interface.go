package repository

import (
	"context"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/postmarkpb"
	"go.mongodb.org/mongo-driver/bson"
	"time"
)

// RoyaltyReportRepositoryInterface is abstraction layer for working with royalty report and representation in database.
type RoyaltyReportRepositoryInterface interface {
	// Insert add the royalty report to the collection.
	Insert(ctx context.Context, document *billingpb.RoyaltyReport, ip, source string) error

	// Update updates the royalty report in the collection.
	Update(ctx context.Context, document *billingpb.RoyaltyReport, ip, source string) error

	// UpdateMany updates multiplies royalty reports by dynamic criteria.
	UpdateMany(ctx context.Context, query bson.M, set bson.M) error

	// GetById returns the royalty report by unique identity.
	GetById(ctx context.Context, id string) (*billingpb.RoyaltyReport, error)

	// GetNonPayoutReports returns the royalty report by unique identity.
	GetNonPayoutReports(ctx context.Context, merchantId, currency string) ([]*billingpb.RoyaltyReport, error)

	// GetByPayoutId returns the royalty report by payout identity.
	GetByPayoutId(ctx context.Context, payoutId string) ([]*billingpb.RoyaltyReport, error)

	// GetBalanceAmount returns royalty balance amount for merchant and currency.
	GetBalanceAmount(ctx context.Context, merchantId, currency string) (float64, error)

	// GetReportExists returns exists a royalty reports by merchant id, currency and dates from/to.
	GetReportExists(ctx context.Context, merchantId, currency string, from, to time.Time) (report *billingpb.RoyaltyReport)

	// GetAll returns the all royalty reports.
	GetAll(ctx context.Context) ([]*billingpb.RoyaltyReport, error)

	// GetByPeriod returns the royalty reports by period of dates.
	GetByPeriod(context.Context, time.Time, time.Time) ([]*billingpb.RoyaltyReport, error)

	// GetByAcceptedExpireWithStatus returns the royalty reports by accepted expire dates with status by filter.
	GetByAcceptedExpireWithStatus(context.Context, time.Time, string) ([]*billingpb.RoyaltyReport, error)

	// GetRoyaltyHistoryById returns the history list of royalty report by report identifier.
	GetRoyaltyHistoryById(ctx context.Context, id string) ([]*billingpb.RoyaltyReportChanges, error)

	// FindByMerchantStatusDates returns the royalty reports by merchant id, status and dates from/to.
	FindByMerchantStatusDates(ctx context.Context, merchantId string, statuses []string, dateFrom, dateTo string, offset, limit int64, sort []string) ([]*billingpb.RoyaltyReport, error)

	// FindCountByMerchantStatusDates returns count of royalty reports by merchant id, status and dates from/to.
	FindCountByMerchantStatusDates(ctx context.Context, merchantId string, statuses []string, dateFrom, dateTo string) (int64, error)

	// Return saved extended form of royalty reports for accountant
	GetAllRoyaltyReportFinanceItems(royaltyReportId string) []*postmarkpb.PayloadAttachment

	// Remove saved extended royalty reports form
	RemoveRoyaltyReportFinanceItems(royaltyReportId string) error

	// Add extended form of royalty reports for accountant to storage
	SetRoyaltyReportFinanceItem(royaltyReportId string, item *postmarkpb.PayloadAttachment) ([]*postmarkpb.PayloadAttachment, error)
}
