package helper

import (
	"go.mongodb.org/mongo-driver/bson"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"math"
)

func MakeOrderAggregateQuery(
	query bson.M,
	lookupCollection string,
	sort []string,
	offset, limit int64,
) []bson.M {

	if limit == 0 {
		limit = math.MaxInt64
	}

	return []bson.M{
		{
			"$match": query,
		},
		{
			"$lookup": bson.M{
				"from":         lookupCollection,
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "order_view",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$order_view",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			// adding only sortable fields here
			"$set": bson.M{
				"report_summary.charge.amount_rounded": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.charge.amount_rounded", bson.M{"$round": []interface{}{"$charge_amount", 2}}}},
				"report_summary.charge.currency":       bson.M{"$ifNull": []interface{}{"$order_view.report_summary.charge.currency", "$charge_currency"}},

				"report_summary.gross.amount_rounded": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.gross.amount_rounded", 0}},

				"report_summary.vat.amount_rounded": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.vat.amount_rounded", 0}},

				"report_summary.fees.amount_rounded": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.fees.amount_rounded", 0}},

				"report_summary.revenue.amount_rounded": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.revenue.amount_rounded", 0}},
			},
		},
		{
			"$sort": mongodb.ToSortOption(sort),
		},
		{
			"$skip": offset,
		},
		{
			"$limit": limit,
		},
		{
			"$set": bson.M{
				"order_charge.amount":         bson.M{"$ifNull": []interface{}{"$order_view.report_summary.charge.amount", "$charge_amount"}},
				"order_charge.amount_rounded": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.charge.amount_rounded", bson.M{"$round": []interface{}{"$charge_amount", 2}}}},
				"order_charge.currency":       bson.M{"$ifNull": []interface{}{"$order_view.report_summary.charge.currency", "$charge_currency"}},

				"report_summary.status": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.status", "$status"}},

				"report_summary.charge.amount": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.charge.amount", "$charge_amount"}},

				"report_summary.gross.amount":   bson.M{"$ifNull": []interface{}{"$order_view.report_summary.gross.amount", 0}},
				"report_summary.gross.currency": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.gross.currency", ""}},

				"report_summary.vat.amount":   bson.M{"$ifNull": []interface{}{"$order_view.report_summary.vat.amount", 0}},
				"report_summary.vat.currency": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.vat.currency", ""}},

				"report_summary.fees.amount":   bson.M{"$ifNull": []interface{}{"$order_view.report_summary.fees.amount", 0}},
				"report_summary.fees.currency": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.fees.currency", ""}},

				"report_summary.revenue.amount":   bson.M{"$ifNull": []interface{}{"$order_view.report_summary.revenue.amount", 0}},
				"report_summary.revenue.currency": bson.M{"$ifNull": []interface{}{"$order_view.report_summary.revenue.currency", ""}},
			},
		},
		{
			"$replaceRoot": bson.M{
				"newRoot": bson.M{
					"$mergeObjects": []interface{}{"$order_view", "$$ROOT"},
				},
			},
		},
		{
			"$project": bson.M{
				"_id":                                   1,
				"uuid":                                  1,
				"total_payment_amount":                  1,
				"currency":                              1,
				"project":                               1,
				"created_at":                            1,
				"pm_order_id":                           1,
				"payment_method":                        1,
				"country_code":                          1,
				"merchant_id":                           1,
				"locale":                                bson.M{"$ifNull": []interface{}{"$user.locale", ""}},
				"status":                                1,
				"pm_order_close_date":                   1,
				"user":                                  1,
				"billing_address":                       1,
				"type":                                  1,
				"is_vat_deduction":                      1,
				"payment_gross_revenue_local":           1,
				"payment_gross_revenue_origin":          1,
				"payment_gross_revenue":                 1,
				"payment_tax_fee":                       1,
				"payment_tax_fee_local":                 1,
				"payment_tax_fee_origin":                1,
				"payment_tax_fee_currency_exchange_fee": 1,
				"payment_tax_fee_total":                 1,
				"payment_gross_revenue_fx":              1,
				"payment_gross_revenue_fx_tax_fee":      1,
				"payment_gross_revenue_fx_profit":       1,
				"gross_revenue":                         1,
				"tax_fee":                               1,
				"tax_fee_currency_exchange_fee":         1,
				"tax_fee_total":                         1,
				"method_fee_total":                      1,
				"method_fee_tariff":                     1,
				"paysuper_method_fee_tariff_self_cost":  1,
				"paysuper_method_fee_profit":            1,
				"method_fixed_fee_tariff":               1,
				"paysuper_method_fixed_fee_tariff_fx_profit":        1,
				"paysuper_method_fixed_fee_tariff_self_cost":        1,
				"paysuper_method_fixed_fee_tariff_total_profit":     1,
				"paysuper_fixed_fee":                                1,
				"paysuper_fixed_fee_fx_profit":                      1,
				"fees_total":                                        1,
				"fees_total_local":                                  1,
				"net_revenue":                                       1,
				"paysuper_method_total_profit":                      1,
				"paysuper_total_profit":                             1,
				"payment_refund_gross_revenue_local":                1,
				"payment_refund_gross_revenue_origin":               1,
				"payment_refund_gross_revenue":                      1,
				"payment_refund_tax_fee":                            1,
				"payment_refund_tax_fee_local":                      1,
				"payment_refund_tax_fee_origin":                     1,
				"payment_refund_fee_tariff":                         1,
				"method_refund_fixed_fee_tariff":                    1,
				"refund_gross_revenue":                              1,
				"refund_gross_revenue_fx":                           1,
				"method_refund_fee_tariff":                          1,
				"paysuper_method_refund_fee_tariff_profit":          1,
				"paysuper_method_refund_fixed_fee_tariff_self_cost": 1,
				"merchant_refund_fixed_fee_tariff":                  1,
				"paysuper_method_refund_fixed_fee_tariff_profit":    1,
				"refund_tax_fee":                                    1,
				"refund_tax_fee_currency_exchange_fee":              1,
				"paysuper_refund_tax_fee_currency_exchange_fee":     1,
				"refund_tax_fee_total":                              1,
				"refund_reverse_revenue":                            1,
				"refund_fees_total":                                 1,
				"refund_fees_total_local":                           1,
				"paysuper_refund_total_profit":                      1,
				"issuer":                                            1,
				"items":                                             1,
				"merchant_payout_currency":                          1,
				"parent_order":                                      1,
				"refund":                                            1,
				"cancellation":                                      1,
				"mcc_code":                                          1,
				"operating_company_id":                              1,
				"is_high_risk":                                      1,
				"refund_allowed":                                    1,
				"vat_payer":                                         1,
				"is_production":                                     1,
				"tax_rate":                                          1,
				"merchant_info":                                     1,
				"order_charge":                                      1,
				"order_charge_before_vat":                           1,
				"payment_method_terminal_id":                        1,
				"metadata":                                          1,
				"metadata_values":                                   1,
				"amount_before_vat":                                 1,
				"royalty_report_id":                                 1,
				"recurring":                                         1,
				"recurring_id":                                      1,
				"report_summary":                                    1,
			},
		},
	}
}
