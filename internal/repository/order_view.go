package repository

import (
	"context"
	"fmt"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	pkg2 "github.com/paysuper/paysuper-billing-server/internal/pkg"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	tools "github.com/paysuper/paysuper-tools/number"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"math"
	"strings"
	"time"
)

const (
	CollectionOrderView = "order_view"
)

type orderViewRepository struct {
	repository
	publicOrderMapper models.Mapper
}

type conformPaylinkStatItemFn func(item *billingpb.StatCommon)

// NewOrderViewRepository create and return an object for working with the order view repository.
// The returned object implements the OrderViewRepositoryInterface interface.
func NewOrderViewRepository(db mongodb.SourceInterface) OrderViewRepositoryInterface {
	s := &orderViewRepository{}
	s.db = db
	s.mapper = models.NewOrderViewPrivateMapper()
	s.publicOrderMapper = models.NewOrderViewPublicMapper()

	return s
}

func (r *orderViewRepository) CountTransactions(ctx context.Context, match bson.M) (n int64, err error) {
	n, err = r.db.Collection(CollectionOrderView).CountDocuments(ctx, match)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, match),
		)
	}
	return
}

func (r *orderViewRepository) GetTransactionsPublic(
	ctx context.Context,
	match bson.M,
	limit, offset int64,
) ([]*billingpb.OrderViewPublic, error) {
	sort := bson.M{"created_at": 1}
	opts := options.Find().
		SetSort(sort).
		SetLimit(limit).
		SetSkip(offset)
	cursor, err := r.db.Collection(CollectionOrderView).Find(ctx, match, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, match),
			zap.Any(pkg.ErrorDatabaseFieldLimit, limit),
			zap.Any(pkg.ErrorDatabaseFieldOffset, offset),
		)
		return nil, err
	}

	var mgoResult []*models.MgoOrderViewPublic
	err = cursor.All(ctx, &mgoResult)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, match),
			zap.Any(pkg.ErrorDatabaseFieldLimit, limit),
			zap.Any(pkg.ErrorDatabaseFieldOffset, offset),
		)
		return nil, err
	}

	grossRevenueAmountMoney := helper.NewMoney()
	vatMoney := helper.NewMoney()
	feeMoney := helper.NewMoney()

	result := make([]*billingpb.OrderViewPublic, len(mgoResult))

	for i, mgo := range mgoResult {
		obj, err := r.publicOrderMapper.MapMgoToObject(mgo)

		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
			)
			return nil, err
		}

		typedObj := obj.(*billingpb.OrderViewPublic)
		grossRevenueAmount, err := grossRevenueAmountMoney.Round(typedObj.GrossRevenue.Amount)

		if err != nil {
			return nil, err
		}

		vat, err := vatMoney.Round(typedObj.TaxFee.Amount)

		if err != nil {
			return nil, err
		}

		fee, err := feeMoney.Round(typedObj.FeesTotal.Amount)

		if err != nil {
			return nil, err
		}

		payoutAmount := grossRevenueAmount - vat - fee

		typedObj.GrossRevenue.Amount = math.Round(grossRevenueAmount*100) / 100
		typedObj.TaxFee.Amount = math.Round(vat*100) / 100
		typedObj.FeesTotal.Amount = math.Round(fee*100) / 100
		typedObj.NetRevenue.Amount = math.Round(payoutAmount*100) / 100
		result[i] = typedObj
	}

	return result, nil
}

func (r *orderViewRepository) GetTransactionsPrivate(
	ctx context.Context,
	match bson.M,
	limit, offset int64,
) ([]*billingpb.OrderViewPrivate, error) {
	sort := bson.M{"created_at": 1}
	opts := options.Find().
		SetSort(sort).
		SetLimit(limit).
		SetSkip(offset)
	cursor, err := r.db.Collection(CollectionOrderView).Find(ctx, match, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, match),
			zap.Any(pkg.ErrorDatabaseFieldLimit, limit),
			zap.Any(pkg.ErrorDatabaseFieldOffset, offset),
		)
		return nil, err
	}

	var mgoResult []*models.MgoOrderViewPrivate
	err = cursor.All(ctx, &mgoResult)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, match),
			zap.Any(pkg.ErrorDatabaseFieldLimit, limit),
			zap.Any(pkg.ErrorDatabaseFieldOffset, offset),
		)
		return nil, err
	}

	result := make([]*billingpb.OrderViewPrivate, len(mgoResult))
	for i, mgo := range mgoResult {
		obj, err := r.mapper.MapMgoToObject(mgo)
		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
			)
			return nil, err
		}
		result[i] = obj.(*billingpb.OrderViewPrivate)
	}

	return result, nil
}

func (r *orderViewRepository) getRoyaltySummaryGroupingQuery(isTotal, needRound bool) []bson.M {
	var groupingId bson.M
	if !isTotal {
		groupingId = bson.M{"product": "$product", "region": "$region"}
	} else {
		groupingId = nil
	}

	result := []bson.M{
		{
			"$group": bson.M{
				"_id":                          groupingId,
				"product":                      bson.M{"$first": "$product"},
				"region":                       bson.M{"$first": "$region"},
				"currency":                     bson.M{"$first": "$currency"},
				"total_transactions":           bson.M{"$sum": 1},
				"gross_sales_amount":           bson.M{"$sum": "$purchase_gross_revenue"},
				"gross_returns_amount":         bson.M{"$sum": "$refund_gross_revenue"},
				"purchase_fees":                bson.M{"$sum": "$purchase_fees_total"},
				"refund_fees":                  bson.M{"$sum": "$refund_fees_total"},
				"purchase_tax":                 bson.M{"$sum": "$purchase_tax_fee_total"},
				"refund_tax":                   bson.M{"$sum": "$refund_tax_fee_total"},
				"net_revenue_total":            bson.M{"$sum": "$net_revenue"},
				"refund_reverse_revenue_total": bson.M{"$sum": "$refund_reverse_revenue"},
				"sales_count":                  bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$type", "order"}}, 1, 0}}},
			},
		},
		{
			"$addFields": bson.M{
				"returns_count":      bson.M{"$subtract": []interface{}{"$total_transactions", "$sales_count"}},
				"gross_total_amount": bson.M{"$subtract": []interface{}{"$gross_sales_amount", "$gross_returns_amount"}},
				"total_fees":         bson.M{"$sum": []interface{}{"$purchase_fees", "$refund_fees"}},
				"total_vat":          bson.M{"$subtract": []interface{}{"$purchase_tax", "$refund_tax"}},
				"payout_amount":      bson.M{"$subtract": []interface{}{"$net_revenue_total", "$refund_reverse_revenue_total"}},
			},
		},
	}

	if needRound {
		round := bson.M{
			"$project": bson.M{
				"_id":                          1,
				"product":                      1,
				"region":                       1,
				"currency":                     1,
				"total_transactions":           1,
				"gross_sales_amount":           bson.M{"$round": []interface{}{"$gross_sales_amount", 2}},
				"gross_returns_amount":         bson.M{"$round": []interface{}{"$gross_returns_amount", 2}},
				"purchase_fees":                bson.M{"$round": []interface{}{"$purchase_fees", 2}},
				"refund_fees":                  bson.M{"$round": []interface{}{"$refund_fees", 2}},
				"purchase_tax":                 bson.M{"$round": []interface{}{"$purchase_tax", 2}},
				"refund_tax":                   bson.M{"$round": []interface{}{"$refund_tax", 2}},
				"net_revenue_total":            bson.M{"$round": []interface{}{"$net_revenue_total", 2}},
				"refund_reverse_revenue_total": bson.M{"$round": []interface{}{"$refund_reverse_revenue_total", 2}},
				"sales_count":                  1,
				"returns_count":                1,
				"gross_total_amount":           bson.M{"$round": []interface{}{"$gross_total_amount", 2}},
				"total_fees":                   bson.M{"$round": []interface{}{"$total_fees", 2}},
				"total_vat":                    bson.M{"$round": []interface{}{"$total_vat", 2}},
				"payout_amount":                bson.M{"$round": []interface{}{"$payout_amount", 2}},
			},
		}

		result = append(result, round)
	}

	result = append(result, bson.M{"$sort": bson.M{"product": 1, "region": 1}})

	return result
}

func (r *orderViewRepository) GetRoyaltySummary(
	ctx context.Context,
	merchantId, currency string,
	from, to time.Time,
) (
	items []*billingpb.RoyaltyReportProductSummaryItem,
	total *billingpb.RoyaltyReportProductSummaryItem,
	orderIds []primitive.ObjectID,
	err error,
) {
	items = []*billingpb.RoyaltyReportProductSummaryItem{}
	total = &billingpb.RoyaltyReportProductSummaryItem{}
	merchantOid, _ := primitive.ObjectIDFromHex(merchantId)

	statusForRoyaltySummary := []string{
		recurringpb.OrderPublicStatusProcessed,
		recurringpb.OrderPublicStatusRefunded,
		recurringpb.OrderPublicStatusChargeback,
	}

	match := bson.M{
		"merchant_id":              merchantOid,
		"merchant_payout_currency": currency,
		"pm_order_close_date":      bson.M{"$gte": from, "$lte": to},
		"status":                   bson.M{"$in": statusForRoyaltySummary},
		"is_production":            true,
	}

	query := []bson.M{
		{
			"$match": match,
		},
		{
			"$project": bson.M{
				"id": "$_id",
				"names": bson.M{
					"$filter": bson.M{
						"input": "$project.name",
						"as":    "name",
						"cond":  bson.M{"$eq": []interface{}{"$$name.lang", "en"}},
					},
				},
				"items": bson.M{
					"$cond": []interface{}{
						bson.M{"$ne": []interface{}{"$items", []interface{}{}}}, "$items", []string{""}},
				},
				"region":                 "$country_code",
				"status":                 1,
				"type":                   1,
				"purchase_gross_revenue": "$gross_revenue.amount",
				"refund_gross_revenue":   "$refund_gross_revenue.amount",
				"purchase_tax_fee_total": "$tax_fee_total.amount",
				"refund_tax_fee_total":   "$refund_tax_fee_total.amount",
				"purchase_fees_total":    "$fees_total.amount",
				"refund_fees_total":      "$refund_fees_total.amount",
				"net_revenue":            "$net_revenue.amount",
				"refund_reverse_revenue": "$refund_reverse_revenue.amount",
				"amount_before_vat":      1,
				"currency":               "$merchant_payout_currency",
			},
		},
		{
			"$unwind": "$items",
		},
		{
			"$project": bson.M{
				"id":                       1,
				"region":                   1,
				"status":                   1,
				"type":                     1,
				"purchase_gross_revenue":   1,
				"refund_gross_revenue":     1,
				"purchase_tax_fee_total":   1,
				"refund_tax_fee_total":     1,
				"purchase_fees_total":      1,
				"refund_fees_total":        1,
				"net_revenue":              1,
				"refund_reverse_revenue":   1,
				"order_amount_without_vat": 1,
				"currency":                 1,
				"product": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []string{"$items", ""}},
						bson.M{"$arrayElemAt": []interface{}{"$names.value", 0}},
						"$items.name",
					},
				},
			},
		},
		{
			"$facet": bson.M{
				"top":        r.getRoyaltySummaryGroupingQuery(false, false),
				"total":      r.getRoyaltySummaryGroupingQuery(true, false),
				"orders_ids": []bson.M{{"$project": bson.M{"id": 1}}},
			},
		},
		{
			"$project": bson.M{
				"top":        "$top",
				"total":      bson.M{"$arrayElemAt": []interface{}{"$total", 0}},
				"orders_ids": "$orders_ids.id",
			},
		},
	}

	cursor, err := r.db.Collection(CollectionOrderView).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, nil, nil, err
	}

	defer func() {
		err := cursor.Close(ctx)
		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorCloseFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			)
		}
	}()

	var result *pkg2.RoyaltySummaryResult
	if cursor.Next(ctx) {
		err = cursor.Decode(&result)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, nil, nil, err
		}
	}

	if result == nil {
		return nil, nil, nil, err
	}

	if result.Items != nil {
		items = result.Items
	}

	if result.Total != nil {
		total = result.Total
		total.Product = ""
		total.Region = ""
	}

	for _, item := range items {
		r.royaltySummaryItemPrecise(item)
	}

	r.royaltySummaryItemPrecise(total)

	orderIds = result.OrdersIds

	return
}

func (r *orderViewRepository) royaltySummaryItemPrecise(item *billingpb.RoyaltyReportProductSummaryItem) {
	item.GrossSalesAmount = tools.ToPrecise(item.GrossSalesAmount)
	item.GrossReturnsAmount = tools.ToPrecise(item.GrossReturnsAmount)
	item.GrossTotalAmount = tools.ToPrecise(item.GrossTotalAmount)
	item.TotalFees = tools.ToPrecise(item.TotalFees)
	item.TotalVat = tools.ToPrecise(item.TotalVat)
	item.PayoutAmount = tools.ToPrecise(item.PayoutAmount)
}

func (r *orderViewRepository) GetPrivateOrderBy(
	ctx context.Context,
	id, uuid, merchantId string,
) (*billingpb.OrderViewPrivate, error) {
	query := bson.M{}

	if id != "" {
		query["_id"], _ = primitive.ObjectIDFromHex(id)
	}

	if uuid != "" {
		query["uuid"] = uuid
	}

	if merchantId != "" {
		query["project.merchant_id"], _ = primitive.ObjectIDFromHex(merchantId)
	}

	aggregateQuery := helper.MakeOrderAggregateQuery(query, CollectionOrderView, []string{}, 0, 1)

	cursor, err := r.db.Collection(CollectionOrder).Aggregate(ctx, aggregateQuery)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, aggregateQuery),
		)
		return nil, err
	}

	var res []*models.MgoOrderViewPrivate

	err = cursor.All(ctx, &res)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, aggregateQuery),
		)
		return nil, err
	}

	if len(res) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	obj, err := r.mapper.MapMgoToObject(res[0])
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, aggregateQuery),
		)
		return nil, err
	}

	return obj.(*billingpb.OrderViewPrivate), nil
}

func (r *orderViewRepository) GetPublicOrderBy(
	ctx context.Context,
	id, uuid, merchantId string,
) (*billingpb.OrderViewPublic, error) {
	query := bson.M{}

	if id != "" {
		query["_id"], _ = primitive.ObjectIDFromHex(id)
	}

	if uuid != "" {
		query["uuid"] = uuid
	}

	if merchantId != "" {
		query["project.merchant_id"], _ = primitive.ObjectIDFromHex(merchantId)
	}

	aggregateQuery := helper.MakeOrderAggregateQuery(query, CollectionOrderView, []string{}, 0, 1)

	cursor, err := r.db.Collection(CollectionOrder).Aggregate(ctx, aggregateQuery)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, aggregateQuery),
		)
		return nil, err
	}

	var res []*models.MgoOrderViewPublic

	err = cursor.All(ctx, &res)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrder),
			zap.Any(pkg.ErrorDatabaseFieldQuery, aggregateQuery),
		)
		return nil, err
	}

	if len(res) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	obj, err := r.publicOrderMapper.MapMgoToObject(res[0])
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, aggregateQuery),
		)
		return nil, err
	}

	return obj.(*billingpb.OrderViewPublic), nil
}

func (r *orderViewRepository) GetPaylinkStatMatchQuery(paylinkId, merchantId string, from, to int64) []bson.M {
	oid, _ := primitive.ObjectIDFromHex(merchantId)
	matchQuery := bson.M{
		"merchant_id":           oid,
		"issuer.reference_type": pkg.OrderIssuerReferenceTypePaylink,
		"issuer.reference":      paylinkId,
	}

	if from > 0 || to > 0 {
		date := bson.M{}
		if from > 0 {
			date["$gte"] = time.Unix(from, 0)
		}
		if to > 0 {
			date["$lte"] = time.Unix(to, 0)
		}
		matchQuery["pm_order_close_date"] = date
	}

	return []bson.M{
		{
			"$match": matchQuery,
		},
	}
}

func (r *orderViewRepository) getPaylinkStatGroupingQuery(groupingId interface{}) []bson.M {
	return []bson.M{
		{
			"$group": bson.M{
				"_id":                   groupingId,
				"total_transactions":    bson.M{"$sum": 1},
				"gross_sales_amount":    bson.M{"$sum": "$payment_gross_revenue.amount"},
				"gross_returns_amount":  bson.M{"$sum": "$refund_gross_revenue.amount"},
				"sales_count":           bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$type", "order"}}, 1, 0}}},
				"country_code":          bson.M{"$first": "$country_code"},
				"transactions_currency": bson.M{"$first": "$merchant_payout_currency"},
			},
		},
		{
			"$addFields": bson.M{
				"returns_count":      bson.M{"$subtract": []interface{}{"$total_transactions", "$sales_count"}},
				"gross_total_amount": bson.M{"$subtract": []interface{}{"$gross_sales_amount", "$gross_returns_amount"}},
			},
		},
	}
}

func (r *orderViewRepository) GetPaylinkStat(
	ctx context.Context,
	paylinkId, merchantId string,
	from, to int64,
) (*billingpb.StatCommon, error) {
	query := append(
		r.GetPaylinkStatMatchQuery(paylinkId, merchantId, from, to),
		r.getPaylinkStatGroupingQuery("$merchant_payout_currency")...,
	)

	var results []*billingpb.StatCommon
	cursor, err := r.db.Collection(CollectionOrderView).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	err = cursor.All(ctx, &results)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	resultsCount := len(results)

	if resultsCount == 1 {
		r.paylinkStatItemPrecise(results[0])
		results[0].PaylinkId = paylinkId
		return results[0], nil
	}

	if resultsCount == 0 {
		return &billingpb.StatCommon{}, nil
	}

	return nil, fmt.Errorf("paylink stat data inconsistent")
}

func (r *orderViewRepository) GetPaylinkStatByCountry(
	ctx context.Context,
	paylinkId, merchantId string,
	from, to int64,
) (result *billingpb.GroupStatCommon, err error) {
	return r.getPaylinkGroupStat(ctx, paylinkId, merchantId, from, to, "$country_code", func(item *billingpb.StatCommon) {
		item.PaylinkId = paylinkId
		item.CountryCode = item.Id
	})
}

func (r *orderViewRepository) GetPaylinkStatByReferrer(
	ctx context.Context,
	paylinkId, merchantId string,
	from, to int64,
) (result *billingpb.GroupStatCommon, err error) {
	return r.getPaylinkGroupStat(ctx, paylinkId, merchantId, from, to, "$issuer.referrer_host", func(item *billingpb.StatCommon) {
		item.PaylinkId = paylinkId
		item.ReferrerHost = item.Id
	})
}

func (r *orderViewRepository) GetPaylinkStatByDate(
	ctx context.Context,
	paylinkId, merchantId string,
	from, to int64,
) (result *billingpb.GroupStatCommon, err error) {
	return r.getPaylinkGroupStat(ctx, paylinkId, merchantId, from, to,
		bson.M{
			"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$pm_order_close_date"},
		},
		func(item *billingpb.StatCommon) {
			item.Date = item.Id
			item.PaylinkId = paylinkId
		})
}

func (r *orderViewRepository) GetPaylinkStatByUtm(
	ctx context.Context,
	paylinkId, merchantId string,
	from, to int64,
) (result *billingpb.GroupStatCommon, err error) {
	return r.getPaylinkGroupStat(ctx, paylinkId, merchantId, from, to,
		bson.M{
			"$concat": []interface{}{"$issuer.utm_source", "&", "$issuer.utm_medium", "&", "$issuer.utm_campaign"},
		},
		func(item *billingpb.StatCommon) {
			if item.Id != "" {
				utm := strings.Split(item.Id, "&")
				item.Utm = &billingpb.Utm{
					UtmSource:   utm[0],
					UtmMedium:   utm[1],
					UtmCampaign: utm[2],
				}
			}
			item.PaylinkId = paylinkId
		})
}

func (r *orderViewRepository) getPaylinkGroupStat(
	ctx context.Context,
	paylinkId string,
	merchantId string,
	from, to int64,
	groupingId interface{},
	conformFn conformPaylinkStatItemFn,
) (result *billingpb.GroupStatCommon, err error) {
	query := r.GetPaylinkStatMatchQuery(paylinkId, merchantId, from, to)
	query = append(query, bson.M{
		"$facet": bson.M{
			"top": append(
				r.getPaylinkStatGroupingQuery(groupingId),
				bson.M{"$sort": bson.M{"_id": 1}},
				bson.M{"$limit": 10},
			),
			"total": r.getPaylinkStatGroupingQuery(""),
		},
	},
		bson.M{
			"$project": bson.M{
				"top":   "$top",
				"total": bson.M{"$arrayElemAt": []interface{}{"$total", 0}},
			},
		})

	cursor, err := r.db.Collection(CollectionOrderView).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer func() {
		err := cursor.Close(ctx)
		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorCloseFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			)
		}
	}()

	if cursor.Next(ctx) {
		err = cursor.Decode(&result)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}
	}

	if result == nil {
		return
	}

	for _, item := range result.Top {
		r.paylinkStatItemPrecise(item)
		conformFn(item)
	}

	if result.Total == nil {
		result.Total = &billingpb.StatCommon{
			PaylinkId: paylinkId,
		}
	}

	r.paylinkStatItemPrecise(result.Total)
	conformFn(result.Total)

	return
}

func (r *orderViewRepository) paylinkStatItemPrecise(item *billingpb.StatCommon) {
	item.GrossSalesAmount = tools.ToPrecise(item.GrossSalesAmount)
	item.GrossReturnsAmount = tools.ToPrecise(item.GrossReturnsAmount)
	item.GrossTotalAmount = tools.ToPrecise(item.GrossTotalAmount)
}

func (r *orderViewRepository) GetPublicByOrderId(ctx context.Context, orderId string) (*billingpb.OrderViewPublic, error) {
	mgo := &models.MgoOrderViewPublic{}
	err := r.db.Collection(CollectionOrderView).FindOne(ctx, bson.M{"uuid": orderId}).Decode(mgo)

	if err != nil {
		return nil, err
	}

	obj, err := r.publicOrderMapper.MapMgoToObject(mgo)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
		)
		return nil, err
	}

	return obj.(*billingpb.OrderViewPublic), nil
}

func (r *orderViewRepository) GetVatSummary(
	ctx context.Context, operatingCompanyId, country string, isVatDeduction bool, from, to time.Time,
) (items []*pkg2.VatReportQueryResItem, err error) {
	matchQuery := bson.M{
		"pm_order_close_date": bson.M{
			"$gte": now.New(from).BeginningOfDay(),
			"$lte": now.New(to).EndOfDay(),
		},
		"country_code":         country,
		"is_vat_deduction":     isVatDeduction,
		"operating_company_id": operatingCompanyId,
		"is_production":        true,
	}

	query := []bson.M{
		{
			"$match": &matchQuery,
		},
		{
			"$group": bson.M{
				"_id":                                "$country_code",
				"count":                              bson.M{"$sum": 1},
				"payment_gross_revenue_local":        bson.M{"$sum": "$payment_gross_revenue_local.amount"},
				"payment_tax_fee_local":              bson.M{"$sum": "$payment_tax_fee_local.amount"},
				"payment_refund_gross_revenue_local": bson.M{"$sum": "$payment_refund_gross_revenue_local.amount"},
				"payment_refund_tax_fee_local":       bson.M{"$sum": "$payment_refund_tax_fee_local.amount"},
				"fees_total":                         bson.M{"$sum": "$fees_total_local.amount"},
				"refund_fees_total":                  bson.M{"$sum": "$refund_fees_total_local.amount"},
			},
		},
	}

	cursor, err := r.db.Collection(CollectionOrderView).Aggregate(ctx, query)

	if err != nil {
		if err != mongo.ErrNoDocuments {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
		}
		return nil, err
	}

	var res []*pkg2.VatReportQueryResItem
	err = cursor.All(ctx, &res)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	return res, nil
}

func (r *orderViewRepository) GetTurnoverSummary(
	ctx context.Context, operatingCompanyId, country, currencyPolicy string, from, to time.Time,
) (items []*pkg2.TurnoverQueryResItem, err error) {
	matchQuery := bson.M{
		"pm_order_close_date": bson.M{
			"$gte": from,
			"$lte": to,
		},
		"operating_company_id": operatingCompanyId,
		"is_production":        true,
		"type":                 pkg.OrderTypeOrder,
		"status":               recurringpb.OrderPublicStatusProcessed,
		"payment_gross_revenue_origin.currency": bson.M{
			"$ne": "",
		},
	}
	if country != "" {
		matchQuery["country_code"] = country
	} else {
		matchQuery["country_code"] = bson.M{"$ne": ""}
	}

	query := []bson.M{
		{
			"$match": matchQuery,
		},
	}

	switch currencyPolicy {
	case pkg.VatCurrencyRatesPolicyOnDay:
		query = append(query, bson.M{
			"$group": bson.M{
				"_id": "$payment_gross_revenue_local.currency",
				"amount": bson.M{
					"$sum": "$payment_gross_revenue_local.amount",
				},
			},
		})
		break

	case pkg.VatCurrencyRatesPolicyLastDay:
		query = append(query, bson.M{
			"$group": bson.M{
				"_id": "$payment_gross_revenue_origin.currency",
				"amount": bson.M{
					"$sum": "$payment_gross_revenue_origin.amount",
				},
			},
		})
		break
	}

	cursor, err := r.db.Collection(CollectionOrderView).Aggregate(ctx, query)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = nil
			return
		}
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return
	}

	var res []*pkg2.TurnoverQueryResItem
	err = cursor.All(ctx, &res)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return
	}

	return res, nil
}

func (r *orderViewRepository) GetRoyaltyForMerchants(
	ctx context.Context, statuses []string, from time.Time, to time.Time,
) ([]*pkg2.RoyaltyReportMerchant, error) {
	var merchants []*pkg2.RoyaltyReportMerchant

	query := []bson.M{
		{
			"$match": bson.M{
				"pm_order_close_date": bson.M{"$gte": from, "$lte": to},
				"status":              bson.M{"$in": statuses},
				"is_production":       true,
			},
		},
		{"$project": bson.M{"project.merchant_id": true}},
		{"$group": bson.M{"_id": "$project.merchant_id"}},
	}

	cursor, err := r.db.Collection(CollectionOrderView).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	err = cursor.All(ctx, &merchants)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	return merchants, nil
}

func (r *orderViewRepository) GetById(ctx context.Context, id string) (*billingpb.OrderViewPublic, error) {
	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	filter := bson.M{"_id": oid}
	mgo := &models.MgoOrderViewPublic{}
	err = r.db.Collection(CollectionOrderView).FindOne(ctx, filter).Decode(mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.String(pkg.ErrorDatabaseFieldDocumentId, id),
		)
		return nil, err
	}

	obj, err := r.publicOrderMapper.MapMgoToObject(mgo)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
		)
		return nil, err
	}

	return obj.(*billingpb.OrderViewPublic), nil
}

func (r *orderViewRepository) GetManyBy(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]*billingpb.OrderViewPrivate, error) {
	var mgo []*models.MgoOrderViewPrivate
	cursor, err := r.db.Collection(CollectionOrderView).Find(ctx, filter, opts...)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	err = cursor.All(ctx, &mgo)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	orders := make([]*billingpb.OrderViewPrivate, len(mgo))
	for i, order := range mgo {
		obj, err := r.mapper.MapMgoToObject(order)
		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
			)
			return nil, err
		}
		orders[i] = obj.(*billingpb.OrderViewPrivate)
	}

	return orders, nil
}

func (r *orderViewRepository) GetRoyaltySummaryRoundedAmounts(
	ctx context.Context,
	merchantId, currency string,
	from, to time.Time,
) (
	items []*billingpb.RoyaltyReportProductSummaryItem,
	total *billingpb.RoyaltyReportProductSummaryItem,
	orderIds []primitive.ObjectID,
	err error,
) {
	items = []*billingpb.RoyaltyReportProductSummaryItem{}
	total = &billingpb.RoyaltyReportProductSummaryItem{}
	merchantOid, _ := primitive.ObjectIDFromHex(merchantId)

	statusForRoyaltySummary := []string{
		recurringpb.OrderPublicStatusProcessed,
		recurringpb.OrderPublicStatusRefunded,
		recurringpb.OrderPublicStatusChargeback,
	}

	match := bson.M{
		"merchant_id":              merchantOid,
		"merchant_payout_currency": currency,
		"pm_order_close_date":      bson.M{"$gte": from, "$lte": to},
		"status":                   bson.M{"$in": statusForRoyaltySummary},
		"is_production":            true,
	}

	query := []bson.M{
		{
			"$match": match,
		},
		{
			"$project": bson.M{
				"id": "$_id",
				"names": bson.M{
					"$filter": bson.M{
						"input": "$project.name",
						"as":    "name",
						"cond":  bson.M{"$eq": []interface{}{"$$name.lang", "en"}},
					},
				},
				"items": bson.M{
					"$cond": []interface{}{
						bson.M{"$ne": []interface{}{"$items", []interface{}{}}}, "$items", []string{""}},
				},
				"region":                 "$country_code",
				"status":                 1,
				"type":                   1,
				"purchase_gross_revenue": "$gross_revenue.amount_rounded",
				"refund_gross_revenue":   "$refund_gross_revenue.amount_rounded",
				"purchase_tax_fee_total": "$tax_fee_total.amount_rounded",
				"refund_tax_fee_total":   "$refund_tax_fee_total.amount_rounded",
				"purchase_fees_total":    "$fees_total.amount_rounded",
				"refund_fees_total":      "$refund_fees_total.amount_rounded",
				"net_revenue":            "$net_revenue.amount_rounded",
				"refund_reverse_revenue": "$refund_reverse_revenue.amount_rounded",
				"amount_before_vat":      1,
				"currency":               "$merchant_payout_currency",
			},
		},
		{
			"$unwind": "$items",
		},
		{
			"$project": bson.M{
				"id":                       1,
				"region":                   1,
				"status":                   1,
				"type":                     1,
				"purchase_gross_revenue":   1,
				"refund_gross_revenue":     1,
				"purchase_tax_fee_total":   1,
				"refund_tax_fee_total":     1,
				"purchase_fees_total":      1,
				"refund_fees_total":        1,
				"net_revenue":              1,
				"refund_reverse_revenue":   1,
				"order_amount_without_vat": 1,
				"currency":                 1,
				"product": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []string{"$items", ""}},
						bson.M{"$arrayElemAt": []interface{}{"$names.value", 0}},
						"$items.name",
					},
				},
			},
		},
		{
			"$project": bson.M{
				"id":                       1,
				"product":                  1,
				"region":                   1,
				"status":                   1,
				"type":                     1,
				"currency":                 1,
				"purchase_gross_revenue":   bson.M{"$round": []interface{}{"$purchase_gross_revenue", 2}},
				"refund_gross_revenue":     bson.M{"$round": []interface{}{"$refund_gross_revenue", 2}},
				"purchase_tax_fee_total":   bson.M{"$round": []interface{}{"$purchase_tax_fee_total", 2}},
				"refund_tax_fee_total":     bson.M{"$round": []interface{}{"$refund_tax_fee_total", 2}},
				"purchase_fees_total":      bson.M{"$round": []interface{}{"$purchase_fees_total", 2}},
				"refund_fees_total":        bson.M{"$round": []interface{}{"$refund_fees_total", 2}},
				"net_revenue":              bson.M{"$round": []interface{}{"$net_revenue", 2}},
				"refund_reverse_revenue":   bson.M{"$round": []interface{}{"$refund_reverse_revenue", 2}},
				"order_amount_without_vat": bson.M{"$round": []interface{}{"$order_amount_without_vat", 2}},
			},
		},
		{
			"$facet": bson.M{
				"top":        r.getRoyaltySummaryGroupingQuery(false, true),
				"total":      r.getRoyaltySummaryGroupingQuery(true, true),
				"orders_ids": []bson.M{{"$project": bson.M{"id": 1}}},
			},
		},
		{
			"$project": bson.M{
				"top":        "$top",
				"total":      bson.M{"$arrayElemAt": []interface{}{"$total", 0}},
				"orders_ids": "$orders_ids.id",
			},
		},
	}

	cursor, err := r.db.Collection(CollectionOrderView).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, nil, nil, err
	}

	defer func() {
		err := cursor.Close(ctx)
		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorCloseFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			)
		}
	}()

	var result *pkg2.RoyaltySummaryResult
	if cursor.Next(ctx) {
		err = cursor.Decode(&result)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, nil, nil, err
		}
	}

	if result == nil {
		return nil, nil, nil, err
	}

	if result.Items != nil {
		items = result.Items
	}

	if result.Total != nil {
		total = result.Total
		total.Product = ""
		total.Region = ""
	}

	for _, item := range items {
		r.royaltySummaryItemPrecise(item)
	}

	r.royaltySummaryItemPrecise(total)

	orderIds = result.OrdersIds

	return
}

func (r *orderViewRepository) GetCountBy(ctx context.Context, filter bson.M, opts ...*options.CountOptions) (int64, error) {
	count, err := r.db.Collection(CollectionOrderView).CountDocuments(ctx, filter, opts...)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return 0, err
	}

	return count, nil
}

func (r *orderViewRepository) FindForRecurringSubscriptions(
	ctx context.Context, userId, merchantId, projectId, subscriptionId, status string, dateFrom, dateTo *time.Time, limit, offset int64,
) (items []*billingpb.OrderViewPrivate, err error) {
	query := bson.M{}

	if userId != "" {
		query["user.id"] = userId
	}

	if merchantId != "" {
		oid, err := primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return nil, err
		}

		query["project.merchant_id"] = oid
	}

	if projectId != "" {
		oid, err := primitive.ObjectIDFromHex(projectId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldQuery, projectId),
			)
			return nil, err
		}

		query["project._id"] = oid
	}

	if subscriptionId != "" {
		query["recurring_subscription_id"] = subscriptionId
	}

	if status != "" {
		query["status"] = status
	}

	if dateFrom != nil && dateTo != nil {
		query["created_at"] = bson.M{"$gte": dateFrom, "$lte": dateTo}
	}

	if limit <= 0 {
		limit = pkg.DatabaseRequestDefaultLimit
	}

	if offset <= 0 {
		offset = 0
	}

	opts := options.Find().
		SetSort(bson.M{"_id": 1}).
		SetLimit(limit).
		SetSkip(offset)

	cursor, err := r.db.Collection(CollectionOrderView).Find(ctx, query, opts)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	var res []*models.MgoOrderViewPrivate

	err = cursor.All(ctx, &res)
	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	orders := make([]*billingpb.OrderViewPrivate, len(res))
	for i, order := range res {
		obj, err := r.mapper.MapMgoToObject(order)
		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, res),
			)
			return nil, err
		}
		orders[i] = obj.(*billingpb.OrderViewPrivate)
	}

	return orders, nil
}

func (r *orderViewRepository) CountForRecurringSubscriptions(
	ctx context.Context, userId, merchantId, projectId, subscriptionId, status string, dateFrom, dateTo *time.Time,
) (count int64, err error) {
	query := bson.M{}

	if userId != "" {
		query["user.id"] = userId
	}

	if merchantId != "" {
		oid, err := primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return 0, err
		}

		query["project.merchant_id"] = oid
	}

	if projectId != "" {
		oid, err := primitive.ObjectIDFromHex(projectId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldQuery, projectId),
			)
			return 0, err
		}

		query["project._id"] = oid
	}

	if subscriptionId != "" {
		query["recurring_subscription_id"] = subscriptionId
	}

	if status != "" {
		query["status"] = status
	}

	if dateFrom != nil && dateTo != nil {
		query["created_at"] = bson.M{"$gte": dateFrom, "$lte": dateTo}
	}

	count, err = r.db.Collection(CollectionOrderView).CountDocuments(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
	}

	return
}
