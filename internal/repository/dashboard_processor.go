package repository

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	pkg2 "github.com/paysuper/paysuper-billing-server/internal/pkg"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"time"
)

const (
	baseReportsItemsLimit = 5
)

type Customers struct {
	Id string `json:"_id" bson:"_id"`
}

type DashboardReportProcessor struct {
	Db          mongodb.SourceInterface
	Collection  string
	Match       bson.M
	GroupBy     string
	DbQueryFn   func(ctx context.Context, receiver interface{}) (interface{}, error)
	Cache       database.CacheInterface
	CacheKey    string
	CacheExpire time.Duration
	Errors      map[string]*billingpb.ResponseErrorMessage
}

func (m *DashboardReportProcessor) ExecuteReport(
	ctx context.Context,
	receiver interface{},
	fn func(context.Context, interface{}) (interface{}, error),
) (interface{}, error) {
	if m.CacheExpire > 0 {
		err := m.Cache.Get(m.CacheKey, &receiver)

		if err == nil {
			return receiver, nil
		}
	}

	m.DbQueryFn = fn
	receiver, err := m.DbQueryFn(ctx, receiver)

	if err != nil {
		return nil, err
	}

	if m.CacheExpire > 0 {
		err = m.Cache.Set(m.CacheKey, receiver, m.CacheExpire)

		if err != nil {
			zap.L().Error(
				pkg.ErrorCacheQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorCacheFieldCmd, "SET"),
				zap.String(pkg.ErrorCacheFieldKey, m.CacheKey),
				zap.Any(pkg.ErrorDatabaseFieldQuery, receiver),
			)

			return nil, err
		}
	}

	return receiver, nil
}

func (m *DashboardReportProcessor) ExecuteGrossRevenueAndVatReports(ctx context.Context, _ interface{}) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"day":                 bson.M{"$dayOfMonth": "$pm_order_close_date"},
				"week":                bson.M{"$week": "$pm_order_close_date"},
				"month":               bson.M{"$month": "$pm_order_close_date"},
				"revenue_amount":      "$payment_gross_revenue.amount",
				"vat_amount":          "$payment_tax_fee.amount",
				"currency":            bson.M{"$ifNull": []string{"$payment_gross_revenue.currency", ""}},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$facet": bson.M{
				"main": []bson.M{
					{
						"$group": bson.M{
							"_id":           nil,
							"gross_revenue": bson.M{"$sum": "$revenue_amount"},
							"currency":      bson.M{"$first": "$currency"},
							"vat_amount":    bson.M{"$sum": "$vat_amount"},
						},
					},
				},
				"chart_gross_revenue": []bson.M{
					{
						"$group": bson.M{
							"_id":   m.GroupBy,
							"label": bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"value": bson.M{"$sum": "$revenue_amount"},
						},
					},
					{"$sort": bson.M{"_id": 1}},
				},
				"chart_vat": []bson.M{
					{
						"$group": bson.M{
							"_id":   m.GroupBy,
							"label": bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"value": bson.M{"$sum": "$vat_amount"},
						},
					},
					{"$sort": bson.M{"_id": 1}},
				},
			},
		},
		{
			"$project": bson.M{
				"gross_revenue": bson.M{
					"amount":   bson.M{"$arrayElemAt": []interface{}{"$main.gross_revenue", 0}},
					"currency": bson.M{"$arrayElemAt": []interface{}{"$main.currency", 0}},
					"chart":    "$chart_gross_revenue",
				},
				"vat": bson.M{
					"amount":   bson.M{"$arrayElemAt": []interface{}{"$main.vat_amount", 0}},
					"currency": bson.M{"$arrayElemAt": []interface{}{"$main.currency", 0}},
					"chart":    "$chart_vat",
				},
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	receiverObj := &pkg2.GrossRevenueAndVatReports{}

	if cursor.Next(ctx) {
		mgo := &models.MgoGrossRevenueAndVatReports{}
		err = cursor.Decode(mgo)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}

		if mgo.GrossRevenue != nil {
			obj, err := models.NewDashboardAmountItemWithChartMapper().MapMgoToObject(mgo.GrossRevenue)

			if err != nil {
				zap.L().Error(
					pkg.ErrorDatabaseMapModelFailed,
					zap.Error(err),
					zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
				)
				return nil, err
			}

			receiverObj.GrossRevenue = obj.(*billingpb.DashboardAmountItemWithChart)
		}

		if mgo.Vat != nil {
			obj, err := models.NewDashboardAmountItemWithChartMapper().MapMgoToObject(mgo.Vat)

			if err != nil {
				zap.L().Error(
					pkg.ErrorDatabaseMapModelFailed,
					zap.Error(err),
					zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
				)
				return nil, err
			}

			receiverObj.Vat = obj.(*billingpb.DashboardAmountItemWithChart)
		}
	}

	return receiverObj, nil
}

func (m *DashboardReportProcessor) ExecuteTotalTransactionsAndArpuReports(ctx context.Context, _ interface{}) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"day":   bson.M{"$dayOfMonth": "$pm_order_close_date"},
				"week":  bson.M{"$week": "$pm_order_close_date"},
				"month": bson.M{"$month": "$pm_order_close_date"},
				"revenue_amount": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []string{"$status", "processed"}}, "$payment_gross_revenue.amount", 0,
					},
				},
				"currency":            bson.M{"$ifNull": []string{"$payment_gross_revenue.currency", ""}},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$facet": bson.M{
				"main": []bson.M{
					{
						"$group": bson.M{
							"_id":                nil,
							"gross_revenue":      bson.M{"$sum": "$revenue_amount"},
							"currency":           bson.M{"$first": "$currency"},
							"total_transactions": bson.M{"$sum": 1},
						},
					},
					{"$addFields": bson.M{"arpu": bson.M{"$divide": []string{"$gross_revenue", "$total_transactions"}}}},
				},
				"chart_total_transactions": []bson.M{
					{
						"$group": bson.M{
							"_id":   m.GroupBy,
							"label": bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"value": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"_id": 1}},
				},
				"chart_arpu": []bson.M{
					{
						"$group": bson.M{
							"_id":                m.GroupBy,
							"label":              bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"gross_revenue":      bson.M{"$sum": "$revenue_amount"},
							"total_transactions": bson.M{"$sum": 1},
						},
					},
					{"$addFields": bson.M{"value": bson.M{"$divide": []string{"$gross_revenue", "$total_transactions"}}}},
					{"$project": bson.M{"label": "$label", "value": "$value"}},
					{"$sort": bson.M{"_id": 1}},
				},
			},
		},
		{
			"$project": bson.M{
				"total_transactions": bson.M{
					"count": bson.M{"$arrayElemAt": []interface{}{"$main.total_transactions", 0}},
					"chart": "$chart_total_transactions",
				},
				"arpu": bson.M{
					"amount":   bson.M{"$arrayElemAt": []interface{}{"$main.arpu", 0}},
					"currency": bson.M{"$arrayElemAt": []interface{}{"$main.currency", 0}},
					"chart":    "$chart_arpu",
				},
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	receiverObj := &pkg2.TotalTransactionsAndArpuReports{}

	if cursor.Next(ctx) {
		mgo := &models.MgoTotalTransactionsAndArpuReports{}
		err = cursor.Decode(mgo)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}

		if mgo.Arpu != nil {
			obj, err := models.NewDashboardAmountItemWithChartMapper().MapMgoToObject(mgo.Arpu)

			if err != nil {
				zap.L().Error(
					pkg.ErrorDatabaseMapModelFailed,
					zap.Error(err),
					zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
				)
				return nil, err
			}

			receiverObj.Arpu = obj.(*billingpb.DashboardAmountItemWithChart)
		}

		if mgo.TotalTransactions != nil {
			receiverObj.TotalTransactions = mgo.TotalTransactions
		}
	}

	return receiverObj, nil
}

func (m *DashboardReportProcessor) ExecuteRevenueDynamicReport(ctx context.Context, receiver interface{}) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"day":                 bson.M{"$dayOfMonth": "$pm_order_close_date"},
				"week":                bson.M{"$week": "$pm_order_close_date"},
				"month":               bson.M{"$month": "$pm_order_close_date"},
				"amount":              "$net_revenue.amount",
				"currency":            bson.M{"$ifNull": []string{"$net_revenue.currency", ""}},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$group": bson.M{
				"_id":      m.GroupBy,
				"label":    bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
				"amount":   bson.M{"$sum": "$amount"},
				"currency": bson.M{"$first": "$currency"},
				"count":    bson.M{"$sum": 1},
			},
		},
		{"$sort": bson.M{"_id": 1}},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	receiverTyped := receiver.(*models.MgoDashboardRevenueDynamicReport)
	err = cursor.All(ctx, &receiverTyped.Items)
	cursor.Close(ctx)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	result := &billingpb.DashboardRevenueDynamicReport{}

	if len(receiverTyped.Items) > 0 {
		for _, item := range receiverTyped.Items {
			obj, err := models.NewDashboardRevenueDynamicReportItemMapper().MapMgoToObject(item)

			if err != nil {
				zap.L().Error(
					pkg.ErrorDatabaseMapModelFailed,
					zap.Error(err),
					zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
				)
				return nil, err
			}

			result.Items = append(result.Items, obj.(*billingpb.DashboardRevenueDynamicReportItem))
		}
	}

	return result, nil
}

func (m *DashboardReportProcessor) ExecuteRevenueByCountryReport(ctx context.Context, _ interface{}) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"hour":  bson.M{"$hour": "$pm_order_close_date"},
				"day":   bson.M{"$dayOfMonth": "$pm_order_close_date"},
				"month": bson.M{"$month": "$pm_order_close_date"},
				"week":  bson.M{"$week": "$pm_order_close_date"},
				"country": bson.M{
					"$cond": []interface{}{
						bson.M{
							"$or": []bson.M{
								{"$eq": []interface{}{"$billing_address", nil}},
								{"$eq": []interface{}{"$billing_address.country", ""}},
							},
						},
						"$user.address.country", "$billing_address.country",
					},
				},
				"amount":              "$net_revenue.amount",
				"currency":            bson.M{"$ifNull": []string{"$net_revenue.currency", ""}},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$project": bson.M{
				"hour":     "$hour",
				"day":      "$day",
				"month":    "$month",
				"week":     "$week",
				"country":  "$country",
				"amount":   "$amount",
				"currency": "$currency",
				"period_in_day": bson.M{
					"$cond": []interface{}{
						bson.M{"$and": []bson.M{{"$gte": []interface{}{"$hour", 0}}, {"$lte": []interface{}{"$hour", 7}}}},
						"00-07",
						bson.M{"$cond": []interface{}{
							bson.M{
								"$and": []bson.M{{"$gte": []interface{}{"$hour", 8}}, {"$lte": []interface{}{"$hour", 15}}},
							}, "08-15", "16-23",
						}},
					},
				},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$project": bson.M{
				"hour":                "$hour",
				"day":                 "$day",
				"month":               "$month",
				"week":                "$week",
				"country":             "$country",
				"amount":              "$amount",
				"currency":            "$currency",
				"period_in_day":       bson.M{"$concat": []interface{}{bson.M{"$toString": "$day"}, " ", "$period_in_day"}},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$facet": bson.M{
				"currency": []bson.M{
					{"$project": bson.M{"currency": "$currency"}},
				},
				"top": []bson.M{
					{
						"$group": bson.M{
							"_id":    "$country",
							"amount": bson.M{"$sum": "$amount"},
						},
					},
					{"$sort": bson.M{"amount": -1}},
					{"$limit": baseReportsItemsLimit},
				},
				"total": []bson.M{
					{
						"$group": bson.M{
							"_id":    nil,
							"amount": bson.M{"$sum": "$amount"},
						},
					},
				},
				"chart": []bson.M{
					{
						"$group": bson.M{
							"_id":    m.GroupBy,
							"label":  bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"amount": bson.M{"$sum": "$amount"},
						},
					},
					{"$sort": bson.M{"label": 1}},
				},
			},
		},
		{
			"$project": bson.M{
				"currency": bson.M{"$arrayElemAt": []interface{}{"$currency.currency", 0}},
				"top":      "$top",
				"total":    bson.M{"$arrayElemAt": []interface{}{"$total.amount", 0}},
				"chart":    "$chart",
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	receiverObj := &billingpb.DashboardRevenueByCountryReport{}

	if cursor.Next(ctx) {
		mgo := &models.MgoDashboardRevenueByCountryReport{}
		err = cursor.Decode(mgo)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}

		obj, err := models.NewDashboardRevenueByCountryReportMapper().MapMgoToObject(mgo)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
			)
			return nil, err
		}

		receiverObj = obj.(*billingpb.DashboardRevenueByCountryReport)
	}

	return receiverObj, nil
}

func (m *DashboardReportProcessor) ExecuteSalesTodayReport(ctx context.Context, receiver interface{}) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"names": bson.M{
					"$filter": bson.M{
						"input": "$project.name",
						"as":    "name",
						"cond":  bson.M{"$eq": []string{"$$name.lang", "en"}},
					},
				},
				"items": bson.M{
					"$cond": []interface{}{
						bson.M{"$ne": []interface{}{"$items", []interface{}{}}}, "$items", []string{""}},
				},
				"hour":                bson.M{"$hour": "$pm_order_close_date"},
				"day":                 bson.M{"$dayOfMonth": "$pm_order_close_date"},
				"month":               bson.M{"$month": "$pm_order_close_date"},
				"week":                bson.M{"$week": "$pm_order_close_date"},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{"$unwind": "$items"},
		{
			"$project": bson.M{
				"item": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []string{"$items", ""}},
						bson.M{"$arrayElemAt": []interface{}{"$names.value", 0}},
						"$items.name",
					},
				},
				"hour":  "$hour",
				"day":   "$day",
				"month": "$month",
				"week":  "$week",
				"period_in_day": bson.M{
					"$cond": []interface{}{
						bson.M{"$and": []bson.M{{"$gte": []interface{}{"$hour", 0}}, {"$lte": []interface{}{"$hour", 7}}}},
						"00-07",
						bson.M{"$cond": []interface{}{
							bson.M{
								"$and": []bson.M{{"$gte": []interface{}{"$hour", 8}}, {"$lte": []interface{}{"$hour", 15}}},
							}, "08-15", "16-23",
						}},
					},
				},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$project": bson.M{
				"item":                "$item",
				"hour":                "$hour",
				"day":                 "$day",
				"month":               "$month",
				"week":                "$week",
				"period_in_day":       bson.M{"$concat": []interface{}{bson.M{"$toString": "$day"}, " ", "$period_in_day"}},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$facet": bson.M{
				"top": []bson.M{
					{
						"$group": bson.M{
							"_id":   "$item",
							"name":  bson.M{"$first": "$item"},
							"count": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"count": -1}},
					{"$limit": baseReportsItemsLimit},
				},
				"total": []bson.M{
					{
						"$group": bson.M{
							"_id":   nil,
							"count": bson.M{"$sum": 1},
						},
					},
				},
				"chart": []bson.M{
					{
						"$group": bson.M{
							"_id":   m.GroupBy,
							"label": bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"value": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"label": 1}},
				},
			},
		},
		{
			"$project": bson.M{
				"top":   "$top",
				"total": bson.M{"$arrayElemAt": []interface{}{"$total.count", 0}},
				"chart": "$chart",
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		err = cursor.Decode(receiver)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}
	}

	return receiver, nil
}

func (m *DashboardReportProcessor) ExecuteSourcesReport(ctx context.Context, receiver interface{}) (interface{}, error) {
	delete(m.Match, "status")
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"hour":                bson.M{"$hour": "$pm_order_close_date"},
				"day":                 bson.M{"$dayOfMonth": "$pm_order_close_date"},
				"month":               bson.M{"$month": "$pm_order_close_date"},
				"week":                bson.M{"$week": "$pm_order_close_date"},
				"issuer":              "$issuer.url",
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$project": bson.M{
				"hour":  "$hour",
				"day":   "$day",
				"month": "$month",
				"week":  "$week",
				"period_in_day": bson.M{
					"$cond": []interface{}{
						bson.M{"$and": []bson.M{{"$gte": []interface{}{"$hour", 0}}, {"$lte": []interface{}{"$hour", 7}}}},
						"00-07",
						bson.M{"$cond": []interface{}{
							bson.M{
								"$and": []bson.M{{"$gte": []interface{}{"$hour", 8}}, {"$lte": []interface{}{"$hour", 15}}},
							}, "08-15", "16-23",
						}},
					},
				},
				"issuer":              "$issuer",
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$project": bson.M{
				"hour":                "$hour",
				"day":                 "$day",
				"month":               "$month",
				"week":                "$week",
				"period_in_day":       bson.M{"$concat": []interface{}{bson.M{"$toString": "$day"}, " ", "$period_in_day"}},
				"issuer":              "$issuer",
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$facet": bson.M{
				"top": []bson.M{
					{
						"$group": bson.M{
							"_id":   "$issuer",
							"name":  bson.M{"$first": "$issuer"},
							"count": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"count": -1}},
					{"$limit": baseReportsItemsLimit},
				},
				"total": []bson.M{
					{
						"$group": bson.M{
							"_id":   nil,
							"count": bson.M{"$sum": 1},
						},
					},
				},
				"chart": []bson.M{
					{
						"$group": bson.M{
							"_id":   m.GroupBy,
							"label": bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"value": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"label": 1}},
				},
			},
		},
		{
			"$project": bson.M{
				"top":   "$top",
				"total": bson.M{"$arrayElemAt": []interface{}{"$total.count", 0}},
				"chart": "$chart",
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		err = cursor.Decode(receiver)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}
	}

	return receiver, nil
}

func (m *DashboardReportProcessor) ExecuteCustomerLTV(ctx context.Context, i interface{}) (interface{}, interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$group": bson.M{
				"_id":      "$user.external_id",
				"sum":      bson.M{"$sum": 1},
				"revenue":  bson.M{"$sum": "$net_revenue.amount"},
				"currency": bson.M{"$max": "$payment_gross_revenue.currency"},
			},
		},
		{
			"$facet": bson.M{
				"result": []bson.M{
					{
						"$group": bson.M{
							"_id":        nil,
							"user_count": bson.M{"$sum": 1},
							"revenue":    bson.M{"$sum": "$revenue"},
							"currency":   bson.M{"$max": "$currency"},
						},
					},
					{
						"$project": bson.M{
							"avg": bson.M{
								"$divide": []string{
									"$revenue", "$user_count",
								},
							},
							"currency": "$currency",
						},
					},
				},
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, nil, err
	}

	defer cursor.Close(ctx)

	type avgOrdersCountItem struct {
		Id       string  `json:"id" bson:"id"`
		Avg      float64 `json:"avg" bson:"avg"`
		Currency string  `json:"currency" bson:"currency"`
	}

	type avgOrdersCountWrapper struct {
		Result []avgOrdersCountItem `json:"result" bson:"result"`
	}
	receiverObj := &avgOrdersCountWrapper{}

	zero := 0.0
	if cursor.Next(ctx) {
		err = cursor.Decode(receiverObj)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, nil, err
		}

		if len(receiverObj.Result) == 0 {
			return zero, "", nil
		}

		return receiverObj.Result[0].Avg, receiverObj.Result[0].Currency, nil
	}

	return zero, "", nil
}

func (m *DashboardReportProcessor) ExecuteCustomerARPU(ctx context.Context, customerId string) (interface{}, error) {
	delete(m.Match, "pm_order_close_date")
	m.Match["user.external_id"] = customerId

	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"day":   bson.M{"$dayOfMonth": "$pm_order_close_date"},
				"week":  bson.M{"$week": "$pm_order_close_date"},
				"month": bson.M{"$month": "$pm_order_close_date"},
				"revenue_amount": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []string{"$status", "processed"}}, "$payment_gross_revenue.amount", 0,
					},
				},
				"currency":            bson.M{"$ifNull": []string{"$payment_gross_revenue.currency", ""}},
				"pm_order_close_date": "$pm_order_close_date",
			},
		},
		{
			"$facet": bson.M{
				"main": []bson.M{
					{
						"$group": bson.M{
							"_id":                nil,
							"gross_revenue":      bson.M{"$sum": "$revenue_amount"},
							"currency":           bson.M{"$first": "$currency"},
							"total_transactions": bson.M{"$sum": 1},
						},
					},
					{"$addFields": bson.M{"arpu": bson.M{"$divide": []string{"$gross_revenue", "$total_transactions"}}}},
				},
				"chart_total_transactions": []bson.M{
					{
						"$group": bson.M{
							"_id":   m.GroupBy,
							"label": bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"value": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"_id": 1}},
				},
				"chart_arpu": []bson.M{
					{
						"$group": bson.M{
							"_id":                m.GroupBy,
							"label":              bson.M{"$last": bson.M{"$toLong": "$pm_order_close_date"}},
							"gross_revenue":      bson.M{"$sum": "$revenue_amount"},
							"total_transactions": bson.M{"$sum": 1},
						},
					},
					{"$addFields": bson.M{"value": bson.M{"$divide": []string{"$gross_revenue", "$total_transactions"}}}},
					{"$project": bson.M{"label": "$label", "value": "$value"}},
					{"$sort": bson.M{"_id": 1}},
				},
			},
		},
		{
			"$project": bson.M{
				"total_transactions": bson.M{
					"count": bson.M{"$arrayElemAt": []interface{}{"$main.total_transactions", 0}},
					"chart": "$chart_total_transactions",
				},
				"arpu": bson.M{
					"amount":   bson.M{"$arrayElemAt": []interface{}{"$main.arpu", 0}},
					"currency": bson.M{"$arrayElemAt": []interface{}{"$main.currency", 0}},
					"chart":    "$chart_arpu",
				},
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	receiverObj := &pkg2.TotalTransactionsAndArpuReports{}

	if cursor.Next(ctx) {
		mgo := &models.MgoTotalTransactionsAndArpuReports{}
		err = cursor.Decode(mgo)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}

		if mgo.Arpu != nil {
			obj, err := models.NewDashboardAmountItemWithChartMapper().MapMgoToObject(mgo.Arpu)

			if err != nil {
				zap.L().Error(
					pkg.ErrorDatabaseMapModelFailed,
					zap.Error(err),
					zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
				)
				return nil, err
			}

			receiverObj.Arpu = obj.(*billingpb.DashboardAmountItemWithChart)
		}

		if mgo.TotalTransactions != nil {
			receiverObj.TotalTransactions = mgo.TotalTransactions
		}
	}

	return receiverObj, nil
}

func (m *DashboardReportProcessor) ExecuteCustomerAvgTransactionsCount(ctx context.Context, i interface{}) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$group": bson.M{
				"_id": "$user.external_id",
				"sum": bson.M{"$sum": 1},
			},
		},
		{
			"$facet": bson.M{
				"result": []bson.M{
					{
						"$group": bson.M{
							"_id":         nil,
							"order_count": bson.M{"$sum": "$sum"},
							"user_count":  bson.M{"$sum": 1},
						},
					},
					{
						"$project": bson.M{
							"avg": bson.M{
								"$divide": []string{
									"$order_count", "$user_count",
								},
							},
						},
					},
				},
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	type avgOrdersCountItem struct {
		Id  string  `json:"id" bson:"id"`
		Avg float64 `json:"avg" bson:"avg"`
	}

	type avgOrdersCountWrapper struct {
		Result []avgOrdersCountItem `json:"result" bson:"result"`
	}
	receiverObj := &avgOrdersCountWrapper{}

	zero := float64(0.0)
	if cursor.Next(ctx) {
		err = cursor.Decode(receiverObj)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return zero, err
		}

		if len(receiverObj.Result) == 0 {
			return zero, nil
		}

		return receiverObj.Result[0].Avg, nil
	}

	return zero, nil
}

func (m *DashboardReportProcessor) ExecuteCustomerTop20(ctx context.Context, i interface{}) (interface{}, error) {
	type top20revenue struct {
		Revenue  float64 `json:"revenue" bson:"revenue"`
		Currency string  `json:"currency" bson:"currency"`
	}

	type top20revenueWrapper struct {
		Result []*top20revenue `json:"result" bson:"result"`
	}
	var receiverObj top20revenueWrapper

	res := &billingpb.Top20Customers{
		Count:    0,
		Revenue:  0,
		Currency: "",
	}

	customersCount, err := m.executeCustomersCount(ctx)
	if err != nil {
		return nil, err
	}

	top20 := int32(customersCount * 0.2)

	if top20 == 0 {
		return res, nil
	}

	query := []bson.M{
		{"$match": m.Match},
		{
			"$group": bson.M{
				"_id":      "$user.external_id",
				"revenue":  bson.M{"$sum": "$net_revenue.amount"},
				"currency": bson.M{"$max": "$payment_gross_revenue.currency"},
			},
		},
		{"$sort": bson.M{"revenue": -1}},
		{"$limit": top20},
		{
			"$facet": bson.M{
				"result": []bson.M{
					{
						"$group": bson.M{
							"_id":      nil,
							"revenue":  bson.M{"$sum": "$revenue"},
							"currency": bson.M{"$max": "$currency"},
						},
					},
				},
			},
		},
	}
	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		err = cursor.Decode(&receiverObj)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return nil, err
		}

		res.Count = top20
		res.Revenue = float32(receiverObj.Result[0].Revenue)
		res.Currency = receiverObj.Result[0].Currency

		return res, nil
	}

	return res, nil
}

func (m *DashboardReportProcessor) ExecuteCustomers(ctx context.Context, i interface{}) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$group": bson.M{
				"_id": "$user.external_id",
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	var receiverObj []*Customers
	err = cursor.All(ctx, &receiverObj)
	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	return receiverObj, nil
}

func (m *DashboardReportProcessor) executeCustomersCount(ctx context.Context) (float32, error) {
	zero := float32(0.0)

	query := []bson.M{
		{"$match": m.Match},
		{
			"$group": bson.M{
				"_id": "$user.external_id",
			},
		},
		{
			"$count": "count",
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return zero, err
	}

	defer cursor.Close(ctx)

	type count struct {
		Count float32 `json:"count" bson:"count"`
	}
	var receiverObj count

	if cursor.Next(ctx) {
		err = cursor.Decode(&receiverObj)

		if err != nil {
			zap.L().Error(
				pkg.ErrorQueryCursorExecutionFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
			return zero, err
		}

		return receiverObj.Count, nil
	}

	return zero, nil
}

func (m *DashboardReportProcessor) ExecuteCustomersCount(ctx context.Context, i interface{}) (interface{}, error) {
	delete(m.Match, "pm_order_close_date")
	return m.executeCustomersCount(ctx)
}

func (m *DashboardReportProcessor) ExecuteCustomersChart(ctx context.Context, startDate time.Time, end time.Time) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"date":    bson.M{"$dateToString": bson.M{"date": "$pm_order_close_date", "format": "%Y-%m-%d"}},
				"user_id": "$user.external_id",
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{"date": "$date", "user": "$user_id"},
			},
		},
		{
			"$group": bson.M{
				"_id":   "$_id.date",
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	type dateChart struct {
		Date  string `json:"_id" bson:"_id"`
		Count int64  `json:"count" bson:"count"`
	}
	var receiverObj []*dateChart

	err = cursor.All(ctx, &receiverObj)
	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	days := int(end.Sub(startDate).Hours() / 24)
	chart := make([]*billingpb.DashboardChartItemInt, days)
	lastDate := startDate.AddDate(0, 0, -1)

	currIndex := 0
	currDate := time.Time{}

	for i := 0; i < days; i++ {
		lastDate = lastDate.AddDate(0, 0, 1)
		val := &billingpb.DashboardChartItemInt{
			Label: lastDate.Unix(),
		}
		chart[i] = val

		if currIndex < len(receiverObj) {
			date := receiverObj[currIndex].Date
			currDate, err = time.Parse("2006-01-02", date)
			if err != nil {
				zap.L().Error("can't parse date", zap.String("date", date), zap.Error(err))
				return nil, err
			}

			if currDate.Equal(lastDate) {
				val.Value = receiverObj[currIndex].Count
				currIndex++
			}
		}
	}

	return chart, nil
}

func (m *DashboardReportProcessor) ExecuteCustomerARPPU(ctx context.Context) (interface{}, error) {
	query := []bson.M{
		{"$match": m.Match},
		{
			"$project": bson.M{
				"user": "$user.external_id",
				"revenue_amount": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []string{"$status", "processed"}}, "$payment_gross_revenue.amount", 0,
					},
				},
				"currency": bson.M{"$ifNull": []string{"$payment_gross_revenue.currency", ""}},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"user":     "$user",
					"currency": "$currency",
				},
				"gross_revenue": bson.M{"$sum": "$revenue_amount"},
			},
		},
		{
			"$group": bson.M{
				"_id":           "$_id.currency",
				"gross_revenue": bson.M{"$sum": "$gross_revenue"},
				"users_count":   bson.M{"$sum": 1},
			},
		},
		{
			"$project": bson.M{
				"currency": "$_id",
				"arppu": bson.M{
					"$divide": []string{
						"$gross_revenue", "$users_count",
					},
				},
			},
		},
	}

	cursor, err := m.Db.Collection(m.Collection).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	type arppu struct {
		Result   float64 `json:"arppu" bson:"arppu"`
		Currency string  `json:"currency" bson:"currency"`
	}
	receiverObj := []*arppu{}

	res := &billingpb.OrderViewMoney{}

	err = cursor.All(ctx, &receiverObj)
	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, m.Collection),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return res, err
	}

	if len(receiverObj) == 0 {
		return res, nil
	}

	if len(receiverObj) > 1 {
		zap.L().Error("data set has more than 1 currency for gross revenue calculating", zap.Any("query", query), zap.Any("result", receiverObj))
	}

	res.AmountRounded = helper.Round(receiverObj[0].Result)
	res.Amount = receiverObj[0].Result
	res.Currency = receiverObj[0].Currency

	return res, nil
}
