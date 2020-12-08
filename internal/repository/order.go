package repository

import (
	"context"
	"errors"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"math"
	"time"
)

const (
	// CollectionOrder is name of table for collection the order.
	CollectionOrder = "order"
)

type orderRepository struct {
	repository
	viewMapper models.Mapper
}

// NewOrderRepository create and return an object for working with the order repository.
// The returned object implements the OrderRepositoryInterface interface.
func NewOrderRepository(db mongodb.SourceInterface) OrderRepositoryInterface {
	s := &orderRepository{
		viewMapper: models.NewOrderViewPrivateMapper(),
	}

	s.mapper = models.NewOrderMapper()
	s.db = db
	return s
}

func (h *orderRepository) GetFirstPaymentForMerchant(ctx context.Context, merchantId string) (*billingpb.Order, error) {
	filter := bson.M{}

	if len(merchantId) == 0 {
		return nil, errors.New("merchant_id must be provided")
	}

	oid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
		return nil, err
	}

	filter["project.merchant_id"] = oid
	filter["status"] = "processed"
	filter["is_production"] = true
	filter["canceled"] = false

	opts := options.FindOne()
	opts.SetSort(bson.D{{"pm_order_close_date", 1}})

	return h.GetOneBy(ctx, filter, opts)
}

func (h *orderRepository) Insert(ctx context.Context, order *billingpb.Order) error {
	mgo, err := h.mapper.MapObjectToMgo(order)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, order),
		)
		return err
	}

	_, err = h.db.Collection(CollectionOrder).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, order),
		)
		return err
	}

	return nil
}

func (h *orderRepository) Update(ctx context.Context, order *billingpb.Order) error {
	oid, err := primitive.ObjectIDFromHex(order.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.String(pkg.ErrorDatabaseFieldQuery, order.Id),
		)
		return err
	}

	mgo, err := h.mapper.MapObjectToMgo(order)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, order),
		)
		return err
	}

	_, err = h.db.Collection(CollectionOrder).ReplaceOne(ctx, bson.M{"_id": oid}, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationUpdate),
			zap.Any(pkg.ErrorDatabaseFieldQuery, order),
		)
		return err
	}

	return nil
}

func (h *orderRepository) GetByUuidAndMerchantId(
	ctx context.Context,
	uuid string,
	merchantId string,
) (*billingpb.Order, error) {
	filter := bson.M{
		"uuid": uuid,
	}

	if merchantId != "" {
		oid, err := primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return nil, err
		}

		filter["project.merchant_id"] = oid
	}

	return h.GetOneBy(ctx, filter)
}

func (h *orderRepository) GetOneBy(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) (*billingpb.Order, error) {
	mgo := &models.MgoOrder{}
	err := h.db.Collection(CollectionOrder).FindOne(ctx, filter, opts...).Decode(mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	obj, err := h.mapper.MapMgoToObject(mgo)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
		)
		return nil, err
	}

	return obj.(*billingpb.Order), nil
}

func (h *orderRepository) GetByUuid(ctx context.Context, uuid string) (*billingpb.Order, error) {
	return h.GetOneBy(ctx, bson.M{"uuid": uuid})
}

func (h *orderRepository) GetById(ctx context.Context, id string) (*billingpb.Order, error) {
	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	return h.GetOneBy(ctx, bson.M{"_id": oid})
}

func (h *orderRepository) GetByRefundReceiptNumber(ctx context.Context, id string) (*billingpb.Order, error) {
	return h.GetOneBy(ctx, bson.M{"refund.receipt_number": id})
}

func (h *orderRepository) UpdateOrderView(ctx context.Context, ids []string) error {
	defer helper.TimeTrack(time.Now(), "updateOrderView")

	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)

		if err != nil {
			zap.L().Error("can't decode ObjectID", zap.Error(err), zap.String("id", id))
			return err
		}

		query := bson.M{"source.id": oid}

		cursor, err := h.db.Collection(collectionAccountingEntry).Find(ctx, query)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionAccountingEntry),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return err
		}

		var list []*models.MgoAccountingEntry
		err = cursor.All(ctx, &list)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionAccountingEntry),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return err
		}

		// empty values
		entries := map[string]*models.MgoAccountingEntry{
			"central_bank_tax_fee":                      {},
			"merchant_method_fee":                       {},
			"merchant_method_fee_cost_value":            {},
			"merchant_method_fixed_fee":                 {},
			"merchant_ps_fixed_fee":                     {},
			"merchant_refund":                           {},
			"merchant_refund_fee":                       {},
			"merchant_refund_fixed_fee":                 {},
			"merchant_refund_fixed_fee_cost_value":      {},
			"merchant_tax_fee_central_bank_fx":          {},
			"merchant_tax_fee_cost_value":               {},
			"ps_gross_revenue_fx":                       {},
			"ps_gross_revenue_fx_tax_fee":               {},
			"ps_method_fee":                             {},
			"ps_reverse_tax_fee_delta":                  {},
			"real_gross_revenue":                        {},
			"real_merchant_method_fixed_fee":            {},
			"real_merchant_method_fixed_fee_cost_value": {},
			"real_merchant_ps_fixed_fee":                {},
			"real_refund":                               {},
			"real_refund_fee":                           {},
			"real_refund_fixed_fee":                     {},
			"real_refund_tax_fee":                       {},
			"real_tax_fee":                              {},
			"reverse_tax_fee":                           {},
			"reverse_tax_fee_delta":                     {},
		}

		// Now setting current values
		for _, entry := range list {
			entries[entry.Type] = entry
		}

		order, err := h.GetById(ctx, id)

		view := &billingpb.OrderViewPrivate{
			Id:                 order.Id,
			Uuid:               order.Uuid,
			TotalPaymentAmount: order.TotalPaymentAmount,
			Currency:           order.Currency,
			Project:            order.Project,
			CreatedAt:          order.CreatedAt,
			Transaction:        order.Transaction,
			PaymentMethod:      order.PaymentMethod,
			CountryCode:        order.CountryCode,
			MerchantId:         order.GetMerchantId(),
			Locale:             "",
			Status:             order.Status,
			TransactionDate:    order.PaymentMethodOrderClosedAt,
			User:               order.User,
			BillingAddress:     order.BillingAddress,
			Type:               order.Type,
			IsVatDeduction:     order.IsVatDeduction,
			PaymentGrossRevenueLocal: &billingpb.OrderViewMoney{
				Amount:        entries["real_gross_revenue"].LocalAmount,
				Currency:      entries["real_gross_revenue"].LocalCurrency,
				AmountRounded: helper.Round(entries["real_gross_revenue"].LocalAmountRounded),
			},
			PaymentGrossRevenueOrigin: &billingpb.OrderViewMoney{
				Amount:        entries["real_gross_revenue"].OriginalAmount,
				Currency:      entries["real_gross_revenue"].OriginalCurrency,
				AmountRounded: helper.Round(entries["real_gross_revenue"].OriginalAmountRounded),
			},
			PaymentGrossRevenue: &billingpb.OrderViewMoney{
				Amount:        entries["real_gross_revenue"].Amount,
				Currency:      entries["real_gross_revenue"].Currency,
				AmountRounded: helper.Round(entries["real_gross_revenue"].AmountRounded),
			},
			PaymentTaxFee: &billingpb.OrderViewMoney{
				Amount:        entries["real_tax_fee"].Amount,
				Currency:      entries["real_tax_fee"].Currency,
				AmountRounded: helper.Round(entries["real_tax_fee"].AmountRounded),
			},
			PaymentTaxFeeLocal: &billingpb.OrderViewMoney{
				Amount:        entries["real_tax_fee"].LocalAmount,
				Currency:      entries["real_tax_fee"].LocalCurrency,
				AmountRounded: helper.Round(entries["real_tax_fee"].LocalAmountRounded),
			},
			PaymentTaxFeeOrigin: &billingpb.OrderViewMoney{
				Amount:        entries["real_tax_fee"].OriginalAmount,
				Currency:      entries["real_tax_fee"].OriginalCurrency,
				AmountRounded: helper.Round(entries["real_tax_fee"].OriginalAmountRounded),
			},
			PaymentTaxFeeCurrencyExchangeFee: &billingpb.OrderViewMoney{
				Amount:        entries["central_bank_tax_fee"].OriginalAmount,
				Currency:      entries["central_bank_tax_fee"].OriginalCurrency,
				AmountRounded: helper.Round(entries["central_bank_tax_fee"].OriginalAmountRounded),
			},
			PaymentTaxFeeTotal: &billingpb.OrderViewMoney{
				Amount:        entries["real_tax_fee"].Amount + entries["central_bank_tax_fee"].Amount,
				Currency:      entries["real_tax_fee"].Currency,
				AmountRounded: helper.Round(entries["real_tax_fee"].AmountRounded + entries["central_bank_tax_fee"].AmountRounded),
			},
			PaymentGrossRevenueFx: &billingpb.OrderViewMoney{
				Amount:        entries["ps_gross_revenue_fx"].Amount,
				Currency:      entries["ps_gross_revenue_fx"].Currency,
				AmountRounded: helper.Round(entries["ps_gross_revenue_fx"].AmountRounded),
			},
			PaymentGrossRevenueFxTaxFee: &billingpb.OrderViewMoney{
				Amount:        entries["ps_gross_revenue_fx_tax_fee"].Amount,
				Currency:      entries["ps_gross_revenue_fx_tax_fee"].Currency,
				AmountRounded: helper.Round(entries["ps_gross_revenue_fx_tax_fee"].AmountRounded),
			},
			PaymentGrossRevenueFxProfit: &billingpb.OrderViewMoney{
				Amount:        entries["ps_gross_revenue_fx"].Amount - entries["ps_gross_revenue_fx_tax_fee"].Amount,
				Currency:      entries["ps_gross_revenue_fx"].Currency,
				AmountRounded: helper.Round(entries["ps_gross_revenue_fx"].AmountRounded - entries["ps_gross_revenue_fx_tax_fee"].AmountRounded),
			},
			GrossRevenue: &billingpb.OrderViewMoney{
				Amount:        entries["real_gross_revenue"].Amount - entries["ps_gross_revenue_fx"].Amount,
				Currency:      entries["real_gross_revenue"].Currency,
				AmountRounded: helper.Round(entries["real_gross_revenue"].AmountRounded - entries["ps_gross_revenue_fx"].AmountRounded),
			},
			TaxFee: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_tax_fee_cost_value"].Amount,
				Currency:      entries["merchant_tax_fee_cost_value"].Currency,
				AmountRounded: helper.Round(entries["merchant_tax_fee_cost_value"].AmountRounded),
			},
			TaxFeeCurrencyExchangeFee: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_tax_fee_central_bank_fx"].Amount,
				Currency:      entries["merchant_tax_fee_central_bank_fx"].Currency,
				AmountRounded: helper.Round(entries["merchant_tax_fee_central_bank_fx"].AmountRounded),
			},
			TaxFeeTotal: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_tax_fee_cost_value"].Amount + entries["merchant_tax_fee_central_bank_fx"].Amount,
				Currency:      entries["merchant_tax_fee_cost_value"].Currency,
				AmountRounded: helper.Round(entries["merchant_tax_fee_cost_value"].AmountRounded + entries["merchant_tax_fee_central_bank_fx"].AmountRounded),
			},
			MethodFeeTotal: &billingpb.OrderViewMoney{
				Amount:        entries["ps_method_fee"].Amount,
				Currency:      entries["ps_method_fee"].Currency,
				AmountRounded: helper.Round(entries["ps_method_fee"].AmountRounded),
			},
			MethodFeeTariff: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_method_fee"].Amount,
				Currency:      entries["merchant_method_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_method_fee"].AmountRounded),
			},
			PaysuperMethodFeeTariffSelfCost: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_method_fee_cost_value"].Amount,
				Currency:      entries["merchant_method_fee_cost_value"].Currency,
				AmountRounded: helper.Round(entries["merchant_method_fee_cost_value"].AmountRounded),
			},
			PaysuperMethodFeeProfit: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_method_fee"].Amount - entries["merchant_method_fee_cost_value"].Amount,
				Currency:      entries["merchant_method_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_method_fee"].AmountRounded - entries["merchant_method_fee_cost_value"].AmountRounded),
			},
			MethodFixedFeeTariff: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_method_fixed_fee"].Amount,
				Currency:      entries["merchant_method_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_method_fixed_fee"].AmountRounded),
			},
			PaysuperMethodFixedFeeTariffFxProfit: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_method_fixed_fee"].Amount - entries["real_merchant_method_fixed_fee"].Amount,
				Currency:      entries["merchant_method_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_method_fixed_fee"].AmountRounded - entries["real_merchant_method_fixed_fee"].AmountRounded),
			},
			PaysuperMethodFixedFeeTariffSelfCost: &billingpb.OrderViewMoney{
				Amount:        entries["real_merchant_method_fixed_fee_cost_value"].Amount,
				Currency:      entries["real_merchant_method_fixed_fee_cost_value"].Currency,
				AmountRounded: helper.Round(entries["real_merchant_method_fixed_fee_cost_value"].AmountRounded),
			},
			PaysuperMethodFixedFeeTariffTotalProfit: &billingpb.OrderViewMoney{
				Amount:        entries["real_merchant_method_fixed_fee"].Amount - entries["real_merchant_method_fixed_fee_cost_value"].Amount,
				Currency:      entries["real_merchant_method_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["real_merchant_method_fixed_fee"].AmountRounded - entries["real_merchant_method_fixed_fee_cost_value"].AmountRounded),
			},
			PaysuperFixedFee: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_ps_fixed_fee"].Amount,
				Currency:      entries["merchant_ps_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_ps_fixed_fee"].AmountRounded),
			},
			PaysuperFixedFeeFxProfit: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_ps_fixed_fee"].Amount - entries["real_merchant_ps_fixed_fee"].Amount,
				Currency:      entries["merchant_ps_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_ps_fixed_fee"].AmountRounded - entries["real_merchant_ps_fixed_fee"].AmountRounded),
			},
			FeesTotal: &billingpb.OrderViewMoney{
				Amount:        entries["ps_method_fee"].Amount + entries["merchant_ps_fixed_fee"].Amount,
				Currency:      entries["ps_method_fee"].Currency,
				AmountRounded: helper.Round(entries["ps_method_fee"].AmountRounded + entries["merchant_ps_fixed_fee"].AmountRounded),
			},
			FeesTotalLocal: &billingpb.OrderViewMoney{
				Amount:        entries["ps_method_fee"].LocalAmount + entries["merchant_ps_fixed_fee"].LocalAmount,
				Currency:      entries["ps_method_fee"].LocalCurrency,
				AmountRounded: helper.Round(entries["ps_method_fee"].LocalAmountRounded + entries["merchant_ps_fixed_fee"].LocalAmountRounded),
			},
			NetRevenue: &billingpb.OrderViewMoney{
				Amount:        entries["real_gross_revenue"].Amount - entries["ps_gross_revenue_fx"].Amount - entries["merchant_ps_fixed_fee"].Amount - entries["merchant_tax_fee_central_bank_fx"].Amount - entries["ps_method_fee"].Amount - entries["merchant_tax_fee_cost_value"].Amount,
				Currency:      entries["real_gross_revenue"].Currency,
				AmountRounded: helper.Round(entries["real_gross_revenue"].AmountRounded - entries["ps_gross_revenue_fx"].AmountRounded - entries["merchant_ps_fixed_fee"].AmountRounded - entries["merchant_tax_fee_central_bank_fx"].AmountRounded - entries["ps_method_fee"].AmountRounded - entries["merchant_tax_fee_cost_value"].AmountRounded),
			},
			PaysuperMethodTotalProfit: &billingpb.OrderViewMoney{
				Amount:        entries["ps_method_fee"].Amount + entries["merchant_ps_fixed_fee"].Amount - entries["merchant_method_fee_cost_value"].Amount - entries["real_merchant_method_fixed_fee_cost_value"].Amount,
				Currency:      entries["ps_method_fee"].Currency,
				AmountRounded: helper.Round(entries["ps_method_fee"].AmountRounded + entries["merchant_ps_fixed_fee"].AmountRounded - entries["merchant_method_fee_cost_value"].AmountRounded - entries["real_merchant_method_fixed_fee_cost_value"].AmountRounded),
			},
			PaysuperTotalProfit: &billingpb.OrderViewMoney{
				Amount:        entries["ps_gross_revenue_fx"].Amount + entries["ps_method_fee"].Amount + entries["merchant_ps_fixed_fee"].Amount - entries["central_bank_tax_fee"].Amount - entries["ps_gross_revenue_fx_tax_fee"].Amount - entries["merchant_method_fee_cost_value"].Amount - entries["real_merchant_method_fixed_fee_cost_value"].Amount,
				Currency:      entries["ps_gross_revenue_fx"].Currency,
				AmountRounded: helper.Round(entries["ps_gross_revenue_fx"].AmountRounded + entries["ps_method_fee"].AmountRounded + entries["merchant_ps_fixed_fee"].AmountRounded - entries["central_bank_tax_fee"].AmountRounded - entries["ps_gross_revenue_fx_tax_fee"].AmountRounded - entries["merchant_method_fee_cost_value"].AmountRounded - entries["real_merchant_method_fixed_fee_cost_value"].AmountRounded),
			},
			PaymentRefundGrossRevenueLocal: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund"].LocalAmount,
				Currency:      entries["real_refund"].LocalCurrency,
				AmountRounded: helper.Round(entries["real_refund"].LocalAmountRounded),
			},
			PaymentRefundGrossRevenueOrigin: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund"].OriginalAmount,
				Currency:      entries["real_refund"].OriginalCurrency,
				AmountRounded: helper.Round(entries["real_refund"].OriginalAmountRounded),
			},
			PaymentRefundGrossRevenue: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund"].Amount,
				Currency:      entries["real_refund"].Currency,
				AmountRounded: helper.Round(entries["real_refund"].AmountRounded),
			},
			PaymentRefundTaxFee: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund_tax_fee"].Amount,
				Currency:      entries["real_refund_tax_fee"].Currency,
				AmountRounded: helper.Round(entries["real_refund_tax_fee"].AmountRounded),
			},
			PaymentRefundTaxFeeLocal: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund_tax_fee"].LocalAmount,
				Currency:      entries["real_refund_tax_fee"].LocalCurrency,
				AmountRounded: helper.Round(entries["real_refund_tax_fee"].LocalAmountRounded),
			},
			PaymentRefundTaxFeeOrigin: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund_tax_fee"].OriginalAmount,
				Currency:      entries["real_refund_tax_fee"].OriginalCurrency,
				AmountRounded: helper.Round(entries["real_refund_tax_fee"].OriginalAmountRounded),
			},
			PaymentRefundFeeTariff: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund_fee"].Amount,
				Currency:      entries["real_refund_fee"].Currency,
				AmountRounded: helper.Round(entries["real_refund_fee"].AmountRounded),
			},
			MethodRefundFixedFeeTariff: &billingpb.OrderViewMoney{
				Amount:        entries["real_refund_fixed_fee"].Amount,
				Currency:      entries["real_refund_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["real_refund_fixed_fee"].AmountRounded),
			},
			RefundGrossRevenue: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund"].Amount,
				Currency:      entries["merchant_refund"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund"].AmountRounded),
			},
			RefundGrossRevenueFx: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund"].Amount - entries["real_refund"].Amount,
				Currency:      entries["merchant_refund"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund"].AmountRounded - entries["real_refund"].AmountRounded),
			},
			MethodRefundFeeTariff: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fee"].Amount,
				Currency:      entries["merchant_refund_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund_fee"].AmountRounded),
			},
			PaysuperMethodRefundFeeTariffProfit: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fee"].Amount - entries["real_refund_fee"].Amount,
				Currency:      entries["merchant_refund_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund_fee"].AmountRounded - entries["real_refund_fee"].AmountRounded),
			},
			PaysuperMethodRefundFixedFeeTariffSelfCost: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fixed_fee_cost_value"].Amount,
				Currency:      entries["merchant_refund_fixed_fee_cost_value"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund_fixed_fee_cost_value"].AmountRounded),
			},
			MerchantRefundFixedFeeTariff: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fixed_fee"].Amount,
				Currency:      entries["merchant_refund_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund_fixed_fee"].AmountRounded),
			},
			PaysuperMethodRefundFixedFeeTariffProfit: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fixed_fee"].Amount - entries["real_refund_fixed_fee"].Amount,
				Currency:      entries["merchant_refund_fixed_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund_fixed_fee"].AmountRounded - entries["real_refund_fixed_fee"].AmountRounded),
			},
			RefundTaxFee: &billingpb.OrderViewMoney{
				Amount:        entries["reverse_tax_fee"].Amount,
				Currency:      entries["reverse_tax_fee"].Currency,
				AmountRounded: helper.Round(entries["reverse_tax_fee"].AmountRounded),
			},
			RefundTaxFeeCurrencyExchangeFee: &billingpb.OrderViewMoney{
				Amount:        entries["reverse_tax_fee_delta"].Amount,
				Currency:      entries["reverse_tax_fee_delta"].Currency,
				AmountRounded: helper.Round(entries["reverse_tax_fee_delta"].AmountRounded),
			},
			PaysuperRefundTaxFeeCurrencyExchangeFee: &billingpb.OrderViewMoney{
				Amount:        entries["ps_reverse_tax_fee_delta"].Amount,
				Currency:      entries["ps_reverse_tax_fee_delta"].Currency,
				AmountRounded: helper.Round(entries["ps_reverse_tax_fee_delta"].AmountRounded),
			},
			RefundTaxFeeTotal: &billingpb.OrderViewMoney{
				Amount:        entries["reverse_tax_fee"].Amount + entries["reverse_tax_fee_delta"].Amount,
				Currency:      entries["reverse_tax_fee"].Currency,
				AmountRounded: helper.Round(entries["reverse_tax_fee"].AmountRounded + entries["reverse_tax_fee_delta"].AmountRounded),
			},
			RefundReverseRevenue: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund"].Amount + entries["merchant_refund_fee"].Amount + entries["merchant_refund_fixed_fee"].Amount + entries["reverse_tax_fee_delta"].Amount - entries["reverse_tax_fee"].Amount,
				Currency:      entries["merchant_refund"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund"].AmountRounded + entries["merchant_refund_fee"].AmountRounded + entries["merchant_refund_fixed_fee"].AmountRounded + entries["reverse_tax_fee_delta"].AmountRounded - entries["reverse_tax_fee"].AmountRounded),
			},
			RefundFeesTotal: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fee"].Amount + entries["merchant_refund_fixed_fee"].Amount,
				Currency:      entries["merchant_refund_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund_fee"].AmountRounded + entries["merchant_refund_fixed_fee"].AmountRounded),
			},
			RefundFeesTotalLocal: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fee"].LocalAmount + entries["merchant_refund_fixed_fee"].LocalAmount,
				Currency:      entries["merchant_refund_fee"].LocalCurrency,
				AmountRounded: helper.Round(entries["merchant_refund_fee"].LocalAmountRounded + entries["merchant_refund_fixed_fee"].LocalAmountRounded),
			},
			PaysuperRefundTotalProfit: &billingpb.OrderViewMoney{
				Amount:        entries["merchant_refund_fee"].Amount + entries["merchant_refund_fixed_fee"].Amount + entries["ps_reverse_tax_fee_delta"].Amount - entries["real_refund_fixed_fee"].Amount - entries["real_refund_fee"].Amount,
				Currency:      entries["merchant_refund_fee"].Currency,
				AmountRounded: helper.Round(entries["merchant_refund_fee"].AmountRounded + entries["merchant_refund_fixed_fee"].AmountRounded + entries["ps_reverse_tax_fee_delta"].AmountRounded - entries["real_refund_fixed_fee"].AmountRounded - entries["real_refund_fee"].AmountRounded),
			},
			Issuer:                 order.Issuer,
			Items:                  order.Items,
			MerchantPayoutCurrency: "",
			ParentOrder:            order.ParentOrder,
			Refund:                 order.Refund,
			Cancellation:           order.Cancellation,
			MccCode:                order.MccCode,
			OperatingCompanyId:     order.OperatingCompanyId,
			IsHighRisk:             order.IsHighRisk,
			RefundAllowed:          order.IsRefundAllowed,
			OrderCharge: &billingpb.OrderViewMoney{
				Amount:        order.ChargeAmount,
				Currency:      order.ChargeCurrency,
				AmountRounded: helper.Round(order.ChargeAmount),
			},
			PaymentIpCountry:            order.PaymentIpCountry,
			IsIpCountryMismatchBin:      order.IsIpCountryMismatchBin,
			BillingCountryChangedByUser: order.BillingCountryChangedByUser,
			VatPayer:                    order.VatPayer,
			IsProduction:                order.IsProduction,
			MerchantInfo:                order.MerchantInfo,
			OrderChargeBeforeVat: &billingpb.OrderViewMoney{
				Amount:        order.ChargeAmount - entries["real_tax_fee"].OriginalAmount - entries["real_refund_tax_fee"].OriginalAmount,
				Currency:      order.ChargeCurrency,
				AmountRounded: helper.Round(order.ChargeAmount - entries["real_tax_fee"].OriginalAmount - entries["real_refund_tax_fee"].OriginalAmount),
			},
			TaxRate:                 order.Tax.Rate,
			PaymentMethodTerminalId: "",
			Recurring:               order.Recurring,
			RecurringId:             order.RecurringId,
			RoyaltyReportId:         order.RoyaltyReportId,
			AmountBeforeVat:         order.OrderAmount,
			MetadataValues:          []string{},
		}

		if len(order.Metadata) > 0 {
			view.MetadataValues = make([]string, 0)

			for _, val := range order.Metadata {
				view.MetadataValues = append(view.MetadataValues, val)
			}
		}

		if order.User != nil {
			view.Locale = order.User.Locale
		}

		if view.Type == pkg.OrderTypeOrder {
			view.MerchantPayoutCurrency = view.NetRevenue.Currency
		} else {
			view.MerchantPayoutCurrency = view.RefundReverseRevenue.Currency
		}

		if order.PaymentMethod != nil && order.PaymentMethod.Params != nil {
			view.PaymentMethodTerminalId = order.PaymentMethod.Params.TerminalId
		}

		// here is some tricky magic:
		// 1.for incoming payment operations right part of expressions always will be zero/empty string
		// 2. and for refunds/chargebacks LEFT part of expressions always will be zero/empty string
		// So, all expressions (both for numbers or strings) always will be safe
		view.ReportSummary = &billingpb.OrderViewReportSummary{
			Status: view.Status,
			Charge: &billingpb.OrderViewMoney{
				Amount:        view.PaymentGrossRevenueOrigin.GetAmount() - view.PaymentRefundGrossRevenueOrigin.GetAmount(),
				Currency:      view.PaymentGrossRevenueOrigin.Currency + view.PaymentRefundGrossRevenueOrigin.Currency,
				AmountRounded: view.PaymentGrossRevenueOrigin.GetAmountRounded() - view.PaymentRefundGrossRevenueOrigin.GetAmountRounded(),
			},
			Gross: &billingpb.OrderViewMoney{
				Amount:        view.GrossRevenue.GetAmount() - view.RefundGrossRevenue.GetAmount(),
				Currency:      view.GrossRevenue.Currency + view.RefundGrossRevenue.Currency,
				AmountRounded: view.GrossRevenue.GetAmountRounded() - view.RefundGrossRevenue.GetAmountRounded(),
			},
			Vat: &billingpb.OrderViewMoney{
				Amount:        view.TaxFeeTotal.GetAmount() - view.RefundTaxFeeTotal.GetAmount(),
				Currency:      view.TaxFeeTotal.Currency + view.RefundTaxFeeTotal.Currency,
				AmountRounded: view.TaxFeeTotal.GetAmountRounded() - view.RefundTaxFeeTotal.GetAmountRounded(),
			},
			Fees: &billingpb.OrderViewMoney{
				Amount:        math.Abs(view.FeesTotal.GetAmount() - view.RefundFeesTotal.GetAmount()),
				Currency:      view.FeesTotal.Currency + view.RefundFeesTotal.Currency,
				AmountRounded: math.Abs(view.FeesTotal.GetAmountRounded() - view.RefundFeesTotal.GetAmountRounded()),
			},
			Revenue: &billingpb.OrderViewMoney{
				Amount:        view.NetRevenue.GetAmount() - view.RefundReverseRevenue.GetAmount(),
				Currency:      view.NetRevenue.Currency + view.RefundReverseRevenue.Currency,
				AmountRounded: view.NetRevenue.GetAmountRounded() - view.RefundReverseRevenue.GetAmountRounded(),
			},
		}

		if view.Type == billingpb.OrderTypeOrder && (view.Status == recurringpb.OrderPublicStatusRefunded || view.Status == recurringpb.OrderPublicStatusChargeback) {
			view.ReportSummary.Status = recurringpb.OrderPublicStatusProcessed
		}

		opts := options.Replace()
		opts.SetUpsert(true)

		viewMgo, err := h.viewMapper.MapObjectToMgo(view)
		if err != nil {
			zap.L().Error("can't map object to mgo", zap.Error(err), zap.String("uuid", view.Uuid))
			return err
		}

		// Here we can update with batches using UpdateMany
		_, err = h.db.Collection(CollectionOrderView).ReplaceOne(ctx, bson.M{"_id": oid}, viewMgo, opts)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationUpdate),
			)
			return err
		}
	}

	return nil
}

func (h *orderRepository) IncludeOrdersToRoyaltyReport(
	ctx context.Context,
	royaltyReportId string,
	orderIds []primitive.ObjectID,
) error {
	filter := bson.M{"_id": bson.M{"$in": orderIds}}
	update := bson.M{"$set": bson.M{"royalty_report_id": royaltyReportId}}
	_, err := h.db.Collection(CollectionOrder).UpdateMany(ctx, filter, update)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
			zap.Any(pkg.ErrorDatabaseFieldSet, update),
		)
	}

	return err
}

func (h *orderRepository) GetManyBy(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]*billingpb.Order, error) {
	var mgo []*models.MgoOrder
	cursor, err := h.db.Collection(CollectionOrder).Find(ctx, filter, opts...)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	err = cursor.All(ctx, &mgo)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	orders := make([]*billingpb.Order, len(mgo))
	for i, order := range mgo {
		obj, err := h.mapper.MapMgoToObject(order)
		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
			)
			return nil, err
		}
		orders[i] = obj.(*billingpb.Order)
	}

	return orders, nil
}
