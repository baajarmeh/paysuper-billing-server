package repository

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/internal/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// OrderViewRepositoryInterface is abstraction layer for working with view of order and representation in database.
type OrderViewRepositoryInterface interface {
	// GetById returns the order view by unique identity.
	GetById(context.Context, string) (*billingpb.OrderViewPublic, error)

	// CountTransactions returns count of transactions by dynamic query.
	CountTransactions(ctx context.Context, match bson.M) (n int64, err error)

	// GetTransactionsPublic returns list of public view transactions by dynamic query.
	GetTransactionsPublic(ctx context.Context, match bson.M, limit, offset int64) (result []*billingpb.OrderViewPublic, err error)

	// GetTransactionsPrivate returns list of private view transactions by dynamic query.
	GetTransactionsPrivate(ctx context.Context, match bson.M, limit, offset int64) (result []*billingpb.OrderViewPrivate, err error)

	// GetRoyaltySummary returns orders for summary royal report by merchant id, currency and dates with checking exists royalty report.
	GetRoyaltySummary(ctx context.Context, merchantId, currency string, from, to time.Time) (items []*billingpb.RoyaltyReportProductSummaryItem, total *billingpb.RoyaltyReportProductSummaryItem, ordersIds []primitive.ObjectID, err error)

	// GetPublicOrderBy returns orders for order identity, order public identity, merchant id.
	GetPublicOrderBy(ctx context.Context, id, uuid, merchantId string) (*billingpb.OrderViewPublic, error)

	// GetPrivateOrderBy returns orders for order identity, order private identity, merchant id.
	GetPrivateOrderBy(ctx context.Context, id, uuid, merchantId string) (*billingpb.OrderViewPrivate, error)

	// GetPaylinkStat returns orders for common paylink report by paylink id, merchant id and dates.
	GetPaylinkStat(ctx context.Context, paylinkId, merchantId string, from, to int64) (*billingpb.StatCommon, error)

	// GetPaylinkStatByCountry returns orders for country paylink report by paylink id, merchant id and dates.
	GetPaylinkStatByCountry(ctx context.Context, paylinkId, merchantId string, from, to int64) (result *billingpb.GroupStatCommon, err error)

	// GetPaylinkStatByReferrer returns orders for referrer paylink report by paylink id, merchant id and dates.
	GetPaylinkStatByReferrer(ctx context.Context, paylinkId, merchantId string, from, to int64) (result *billingpb.GroupStatCommon, err error)

	// GetPaylinkStatByDate returns orders for dates paylink report by paylink id, merchant id and dates.
	GetPaylinkStatByDate(ctx context.Context, paylinkId, merchantId string, from, to int64) (result *billingpb.GroupStatCommon, err error)

	// GetPaylinkStatByUtm returns orders for utm paylink report by paylink id, merchant id and dates.
	GetPaylinkStatByUtm(ctx context.Context, paylinkId, merchantId string, from, to int64) (result *billingpb.GroupStatCommon, err error)

	// GetPublicByOrderId returns the public order view by unique identity.
	GetPublicByOrderId(ctx context.Context, merchantId string) (*billingpb.OrderViewPublic, error)

	// GetVatSummary returns orders for summary vat report by operating company id, country, vat deduction and dates.
	GetVatSummary(context.Context, string, string, bool, time.Time, time.Time) ([]*pkg.VatReportQueryResItem, error)

	// GetTurnoverSummary returns orders for summary turnover report by operating company id, country, currency policy and dates.
	GetTurnoverSummary(context.Context, string, string, string, time.Time, time.Time) ([]*pkg.TurnoverQueryResItem, error)

	// GetRoyaltyForMerchants returns orders for merchants royal report by statuses and dates.
	GetRoyaltyForMerchants(context.Context, []string, time.Time, time.Time) ([]*pkg.RoyaltyReportMerchant, error)

	// Return orders by some conditions and with options
	GetManyBy(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]*billingpb.OrderViewPrivate, error)

	// Return count of orders by some conditions and with options
	GetCountBy(ctx context.Context, filter bson.M, opts ...*options.CountOptions) (int64, error)

	// GetRoyaltySummary returns orders for summary royal report by merchant id, currency and dates with checking exists royalty report.
	GetRoyaltySummaryRoundedAmounts(ctx context.Context, merchantId, currency string, from, to time.Time) (items []*billingpb.RoyaltyReportProductSummaryItem, total *billingpb.RoyaltyReportProductSummaryItem, ordersIds []primitive.ObjectID, err error)

	// FindForRecurringSubscriptions returns orders by subscription params.
	FindForRecurringSubscriptions(ctx context.Context, userId, merchantId, projectId, subscriptionId, status string, dateFrom, dateTo *time.Time, limit, offset int64) (items []*billingpb.OrderViewPrivate, err error)

	// CountForRecurringSubscriptions returns count orders by subscription params.
	CountForRecurringSubscriptions(ctx context.Context, userId, merchantId, projectId, subscriptionId, status string, dateFrom, dateTo *time.Time) (count int64, err error)
}
