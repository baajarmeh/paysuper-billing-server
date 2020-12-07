package models

import (
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type MgoOrderViewPrivate struct {
	Id                                         primitive.ObjectID                       `bson:"_id" json:"-" faker:"objectId"`
	Uuid                                       string                                   `bson:"uuid" json:"uuid"`
	TotalPaymentAmount                         float64                                  `bson:"total_payment_amount" json:"total_payment_amount"`
	Currency                                   string                                   `bson:"currency" json:"currency"`
	Project                                    *MgoOrderProject                         `bson:"project" json:"project"`
	CreatedAt                                  time.Time                                `bson:"created_at" json:"created_at"`
	Transaction                                string                                   `bson:"pm_order_id" json:"transaction"`
	PaymentMethod                              *MgoOrderPaymentMethod                   `bson:"payment_method" json:"payment_method"`
	CountryCode                                string                                   `bson:"country_code" json:"country_code"`
	MerchantId                                 primitive.ObjectID                       `bson:"merchant_id" json:"merchant_id" faker:"objectId"`
	Locale                                     string                                   `bson:"locale" json:"locale"`
	Status                                     string                                   `bson:"status" json:"status"`
	TransactionDate                            time.Time                                `bson:"pm_order_close_date" json:"transaction_date"`
	User                                       *billingpb.OrderUser                     `bson:"user" json:"user"`
	BillingAddress                             *billingpb.OrderBillingAddress           `bson:"billing_address" json:"billing_address"`
	Type                                       string                                   `bson:"type" json:"type"`
	IsVatDeduction                             bool                                     `bson:"is_vat_deduction" json:"is_vat_deduction"`
	PaymentGrossRevenueLocal                   *billingpb.OrderViewMoney                `bson:"payment_gross_revenue_local" json:"payment_gross_revenue_local"`
	PaymentGrossRevenueOrigin                  *billingpb.OrderViewMoney                `bson:"payment_gross_revenue_origin" json:"payment_gross_revenue_origin"`
	PaymentGrossRevenue                        *billingpb.OrderViewMoney                `bson:"payment_gross_revenue" json:"payment_gross_revenue"`
	PaymentTaxFee                              *billingpb.OrderViewMoney                `bson:"payment_tax_fee" json:"payment_tax_fee"`
	PaymentTaxFeeLocal                         *billingpb.OrderViewMoney                `bson:"payment_tax_fee_local" json:"payment_tax_fee_local"`
	PaymentTaxFeeOrigin                        *billingpb.OrderViewMoney                `bson:"payment_tax_fee_origin" json:"payment_tax_fee_origin"`
	PaymentTaxFeeCurrencyExchangeFee           *billingpb.OrderViewMoney                `bson:"payment_tax_fee_currency_exchange_fee" json:"payment_tax_fee_currency_exchange_fee"`
	PaymentTaxFeeTotal                         *billingpb.OrderViewMoney                `bson:"payment_tax_fee_total" json:"payment_tax_fee_total"`
	PaymentGrossRevenueFx                      *billingpb.OrderViewMoney                `bson:"payment_gross_revenue_fx" json:"payment_gross_revenue_fx"`
	PaymentGrossRevenueFxTaxFee                *billingpb.OrderViewMoney                `bson:"payment_gross_revenue_fx_tax_fee" json:"payment_gross_revenue_fx_tax_fee"`
	PaymentGrossRevenueFxProfit                *billingpb.OrderViewMoney                `bson:"payment_gross_revenue_fx_profit" json:"payment_gross_revenue_fx_profit"`
	GrossRevenue                               *billingpb.OrderViewMoney                `bson:"gross_revenue" json:"gross_revenue"`
	TaxFee                                     *billingpb.OrderViewMoney                `bson:"tax_fee" json:"tax_fee"`
	TaxFeeCurrencyExchangeFee                  *billingpb.OrderViewMoney                `bson:"tax_fee_currency_exchange_fee" json:"tax_fee_currency_exchange_fee"`
	TaxFeeTotal                                *billingpb.OrderViewMoney                `bson:"tax_fee_total" json:"tax_fee_total"`
	MethodFeeTotal                             *billingpb.OrderViewMoney                `bson:"method_fee_total" json:"method_fee_total"`
	MethodFeeTariff                            *billingpb.OrderViewMoney                `bson:"method_fee_tariff" json:"method_fee_tariff"`
	PaysuperMethodFeeTariffSelfCost            *billingpb.OrderViewMoney                `bson:"paysuper_method_fee_tariff_self_cost" json:"paysuper_method_fee_tariff_self_cost"`
	PaysuperMethodFeeProfit                    *billingpb.OrderViewMoney                `bson:"paysuper_method_fee_profit" json:"paysuper_method_fee_profit"`
	MethodFixedFeeTariff                       *billingpb.OrderViewMoney                `bson:"method_fixed_fee_tariff" json:"method_fixed_fee_tariff"`
	PaysuperMethodFixedFeeTariffFxProfit       *billingpb.OrderViewMoney                `bson:"paysuper_method_fixed_fee_tariff_fx_profit" json:"paysuper_method_fixed_fee_tariff_fx_profit"`
	PaysuperMethodFixedFeeTariffSelfCost       *billingpb.OrderViewMoney                `bson:"paysuper_method_fixed_fee_tariff_self_cost" json:"paysuper_method_fixed_fee_tariff_self_cost"`
	PaysuperMethodFixedFeeTariffTotalProfit    *billingpb.OrderViewMoney                `bson:"paysuper_method_fixed_fee_tariff_total_profit" json:"paysuper_method_fixed_fee_tariff_total_profit"`
	PaysuperFixedFee                           *billingpb.OrderViewMoney                `bson:"paysuper_fixed_fee" json:"paysuper_fixed_fee"`
	PaysuperFixedFeeFxProfit                   *billingpb.OrderViewMoney                `bson:"paysuper_fixed_fee_fx_profit" json:"paysuper_fixed_fee_fx_profit"`
	FeesTotal                                  *billingpb.OrderViewMoney                `bson:"fees_total" json:"fees_total"`
	FeesTotalLocal                             *billingpb.OrderViewMoney                `bson:"fees_total_local" json:"fees_total_local"`
	NetRevenue                                 *billingpb.OrderViewMoney                `bson:"net_revenue" json:"net_revenue"`
	PaysuperMethodTotalProfit                  *billingpb.OrderViewMoney                `bson:"paysuper_method_total_profit" json:"paysuper_method_total_profit"`
	PaysuperTotalProfit                        *billingpb.OrderViewMoney                `bson:"paysuper_total_profit" json:"paysuper_total_profit"`
	PaymentRefundGrossRevenueLocal             *billingpb.OrderViewMoney                `bson:"payment_refund_gross_revenue_local" json:"payment_refund_gross_revenue_local"`
	PaymentRefundGrossRevenueOrigin            *billingpb.OrderViewMoney                `bson:"payment_refund_gross_revenue_origin" json:"payment_refund_gross_revenue_origin"`
	PaymentRefundGrossRevenue                  *billingpb.OrderViewMoney                `bson:"payment_refund_gross_revenue" json:"payment_refund_gross_revenue"`
	PaymentRefundTaxFee                        *billingpb.OrderViewMoney                `bson:"payment_refund_tax_fee" json:"payment_refund_tax_fee"`
	PaymentRefundTaxFeeLocal                   *billingpb.OrderViewMoney                `bson:"payment_refund_tax_fee_local" json:"payment_refund_tax_fee_local"`
	PaymentRefundTaxFeeOrigin                  *billingpb.OrderViewMoney                `bson:"payment_refund_tax_fee_origin" json:"payment_refund_tax_fee_origin"`
	PaymentRefundFeeTariff                     *billingpb.OrderViewMoney                `bson:"payment_refund_fee_tariff" json:"payment_refund_fee_tariff"`
	MethodRefundFixedFeeTariff                 *billingpb.OrderViewMoney                `bson:"method_refund_fixed_fee_tariff" json:"method_refund_fixed_fee_tariff"`
	RefundGrossRevenue                         *billingpb.OrderViewMoney                `bson:"refund_gross_revenue" json:"refund_gross_revenue"`
	RefundGrossRevenueFx                       *billingpb.OrderViewMoney                `bson:"refund_gross_revenue_fx" json:"refund_gross_revenue_fx"`
	MethodRefundFeeTariff                      *billingpb.OrderViewMoney                `bson:"method_refund_fee_tariff" json:"method_refund_fee_tariff"`
	PaysuperMethodRefundFeeTariffProfit        *billingpb.OrderViewMoney                `bson:"paysuper_method_refund_fee_tariff_profit" json:"paysuper_method_refund_fee_tariff_profit"`
	PaysuperMethodRefundFixedFeeTariffSelfCost *billingpb.OrderViewMoney                `bson:"paysuper_method_refund_fixed_fee_tariff_self_cost" json:"paysuper_method_refund_fixed_fee_tariff_self_cost"`
	MerchantRefundFixedFeeTariff               *billingpb.OrderViewMoney                `bson:"merchant_refund_fixed_fee_tariff" json:"merchant_refund_fixed_fee_tariff"`
	PaysuperMethodRefundFixedFeeTariffProfit   *billingpb.OrderViewMoney                `bson:"paysuper_method_refund_fixed_fee_tariff_profit" json:"paysuper_method_refund_fixed_fee_tariff_profit"`
	RefundTaxFee                               *billingpb.OrderViewMoney                `bson:"refund_tax_fee" json:"refund_tax_fee"`
	RefundTaxFeeCurrencyExchangeFee            *billingpb.OrderViewMoney                `bson:"refund_tax_fee_currency_exchange_fee" json:"refund_tax_fee_currency_exchange_fee"`
	PaysuperRefundTaxFeeCurrencyExchangeFee    *billingpb.OrderViewMoney                `bson:"paysuper_refund_tax_fee_currency_exchange_fee" json:"paysuper_refund_tax_fee_currency_exchange_fee"`
	RefundTaxFeeTotal                          *billingpb.OrderViewMoney                `bson:"refund_tax_fee_total" json:"refund_tax_fee_total"`
	RefundReverseRevenue                       *billingpb.OrderViewMoney                `bson:"refund_reverse_revenue" json:"refund_reverse_revenue"`
	RefundFeesTotal                            *billingpb.OrderViewMoney                `bson:"refund_fees_total" json:"refund_fees_total"`
	RefundFeesTotalLocal                       *billingpb.OrderViewMoney                `bson:"refund_fees_total_local" json:"refund_fees_total_local"`
	PaysuperRefundTotalProfit                  *billingpb.OrderViewMoney                `bson:"paysuper_refund_total_profit" json:"paysuper_refund_total_profit"`
	Issuer                                     *billingpb.OrderIssuer                   `bson:"issuer"`
	Items                                      []*MgoOrderItem                          `bson:"items"`
	MerchantPayoutCurrency                     string                                   `bson:"merchant_payout_currency"`
	ParentOrder                                *billingpb.ParentOrder                   `bson:"parent_order"`
	Refund                                     *MgoOrderNotificationRefund              `bson:"refund"`
	Cancellation                               *billingpb.OrderNotificationCancellation `bson:"cancellation"`
	MccCode                                    string                                   `bson:"mcc_code"`
	OperatingCompanyId                         string                                   `bson:"operating_company_id"`
	IsHighRisk                                 bool                                     `bson:"is_high_risk"`
	RefundAllowed                              bool                                     `bson:"refund_allowed"`
	VatPayer                                   string                                   `bson:"vat_payer"`
	IsProduction                               bool                                     `bson:"is_production"`
	TaxRate                                    float64                                  `bson:"tax_rate"`
	MerchantInfo                               *billingpb.OrderViewMerchantInfo         `bson:"merchant_info"`
	OrderCharge                                *billingpb.OrderViewMoney                `bson:"order_charge"`
	OrderChargeBeforeVat                       *billingpb.OrderViewMoney                `bson:"order_charge_before_vat"`
	PaymentMethodTerminalId                    string                                   `bson:"payment_method_terminal_id"`
	MetadataValues                             []string                                 `bson:"metadata_values"`
	AmountBeforeVat                            float64                                  `bson:"amount_before_vat"`
	RoyaltyReportId                            string                                   `bson:"royalty_report_id"`
	Recurring                                  bool                                     `bson:"recurring"`
	RecurringId                                string                                   `bson:"recurring_id"`
	ReportSummary                              *MgoOrderViewReportSummary               `bson:"report_summary" json:"report_summary"`
}

type orderViewPrivateMapper struct{}

func NewOrderViewPrivateMapper() Mapper {
	return &orderViewPrivateMapper{}
}

func (o *orderViewPrivateMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.OrderViewPrivate)

	oid, err := primitive.ObjectIDFromHex(in.Id)
	if err != nil {
		return nil, err
	}

	merchantOid, err := primitive.ObjectIDFromHex(in.MerchantId)
	if err != nil {
		return nil, err
	}

	projectOid, err := primitive.ObjectIDFromHex(in.Project.Id)
	if err != nil {
		return nil, err
	}

	out := &MgoOrderViewPrivate{
		Id:                 oid,
		Uuid:               in.Uuid,
		TotalPaymentAmount: in.TotalPaymentAmount,
		Currency:           in.Currency,
		Project: &MgoOrderProject{
			Id:                      projectOid,
			MerchantId:              merchantOid,
			UrlSuccess:              in.Project.UrlSuccess,
			UrlFail:                 in.Project.UrlFail,
			NotifyEmails:            in.Project.NotifyEmails,
			SendNotifyEmail:         in.Project.SendNotifyEmail,
			SecretKey:               in.Project.SecretKey,
			UrlCheckAccount:         in.Project.UrlCheckAccount,
			UrlProcessPayment:       in.Project.UrlProcessPayment,
			CallbackProtocol:        in.Project.CallbackProtocol,
			UrlChargebackPayment:    in.Project.UrlChargebackPayment,
			UrlCancelPayment:        in.Project.UrlCancelPayment,
			UrlRefundPayment:        in.Project.UrlRefundPayment,
			UrlFraudPayment:         in.Project.UrlFraudPayment,
			Status:                  in.Project.Status,
			MerchantRoyaltyCurrency: in.Project.MerchantRoyaltyCurrency,
			RedirectSettings:        in.Project.RedirectSettings,
		},
		Transaction:                                in.Transaction,
		PaymentMethod:                              nil,
		CountryCode:                                in.CountryCode,
		MerchantId:                                 merchantOid,
		Locale:                                     in.Locale,
		Status:                                     in.Status,
		User:                                       in.User,
		BillingAddress:                             in.BillingAddress,
		Type:                                       in.Type,
		IsVatDeduction:                             in.IsVatDeduction,
		PaymentGrossRevenueLocal:                   in.PaymentGrossRevenueLocal,
		PaymentGrossRevenueOrigin:                  in.PaymentGrossRevenueOrigin,
		PaymentGrossRevenue:                        in.PaymentGrossRevenue,
		PaymentTaxFee:                              in.PaymentTaxFee,
		PaymentTaxFeeLocal:                         in.PaymentTaxFeeLocal,
		PaymentTaxFeeOrigin:                        in.PaymentTaxFeeOrigin,
		PaymentTaxFeeCurrencyExchangeFee:           in.PaymentTaxFeeCurrencyExchangeFee,
		PaymentTaxFeeTotal:                         in.PaymentTaxFeeTotal,
		PaymentGrossRevenueFx:                      in.PaymentGrossRevenueFx,
		PaymentGrossRevenueFxTaxFee:                in.PaymentGrossRevenueFxTaxFee,
		PaymentGrossRevenueFxProfit:                in.PaymentGrossRevenueFxProfit,
		GrossRevenue:                               in.GrossRevenue,
		TaxFee:                                     in.TaxFee,
		TaxFeeCurrencyExchangeFee:                  in.TaxFeeCurrencyExchangeFee,
		TaxFeeTotal:                                in.TaxFeeTotal,
		MethodFeeTotal:                             in.MethodFeeTotal,
		MethodFeeTariff:                            in.MethodFeeTariff,
		PaysuperMethodFeeTariffSelfCost:            in.PaysuperMethodFeeTariffSelfCost,
		PaysuperMethodFeeProfit:                    in.PaysuperMethodFeeProfit,
		MethodFixedFeeTariff:                       in.MethodFixedFeeTariff,
		PaysuperMethodFixedFeeTariffFxProfit:       in.PaysuperMethodFixedFeeTariffFxProfit,
		PaysuperMethodFixedFeeTariffSelfCost:       in.PaysuperMethodFixedFeeTariffSelfCost,
		PaysuperMethodFixedFeeTariffTotalProfit:    in.PaysuperMethodFixedFeeTariffTotalProfit,
		PaysuperFixedFee:                           in.PaysuperFixedFee,
		PaysuperFixedFeeFxProfit:                   in.PaysuperFixedFeeFxProfit,
		FeesTotal:                                  in.FeesTotal,
		FeesTotalLocal:                             in.FeesTotalLocal,
		NetRevenue:                                 in.NetRevenue,
		PaysuperMethodTotalProfit:                  in.PaysuperMethodTotalProfit,
		PaysuperTotalProfit:                        in.PaysuperTotalProfit,
		PaymentRefundGrossRevenueLocal:             in.PaymentRefundGrossRevenueLocal,
		PaymentRefundGrossRevenueOrigin:            in.PaymentRefundGrossRevenueOrigin,
		PaymentRefundGrossRevenue:                  in.PaymentRefundGrossRevenue,
		PaymentRefundTaxFee:                        in.PaymentRefundTaxFee,
		PaymentRefundTaxFeeLocal:                   in.PaymentRefundTaxFeeLocal,
		PaymentRefundTaxFeeOrigin:                  in.PaymentRefundTaxFeeOrigin,
		PaymentRefundFeeTariff:                     in.PaymentRefundFeeTariff,
		MethodRefundFixedFeeTariff:                 in.MethodRefundFixedFeeTariff,
		RefundGrossRevenue:                         in.RefundGrossRevenue,
		RefundGrossRevenueFx:                       in.RefundGrossRevenueFx,
		MethodRefundFeeTariff:                      in.MethodRefundFeeTariff,
		PaysuperMethodRefundFeeTariffProfit:        in.PaysuperMethodRefundFeeTariffProfit,
		PaysuperMethodRefundFixedFeeTariffSelfCost: in.PaysuperMethodRefundFixedFeeTariffSelfCost,
		MerchantRefundFixedFeeTariff:               in.MerchantRefundFixedFeeTariff,
		PaysuperMethodRefundFixedFeeTariffProfit:   in.PaysuperMethodRefundFixedFeeTariffProfit,
		RefundTaxFee:                               in.RefundTaxFee,
		RefundTaxFeeCurrencyExchangeFee:            in.RefundTaxFeeCurrencyExchangeFee,
		PaysuperRefundTaxFeeCurrencyExchangeFee:    in.PaysuperRefundTaxFeeCurrencyExchangeFee,
		RefundTaxFeeTotal:                          in.RefundTaxFeeTotal,
		RefundReverseRevenue:                       in.RefundReverseRevenue,
		RefundFeesTotal:                            in.RefundFeesTotal,
		RefundFeesTotalLocal:                       in.RefundFeesTotalLocal,
		PaysuperRefundTotalProfit:                  in.PaysuperRefundTotalProfit,
		Issuer:                                     in.Issuer,
		MerchantPayoutCurrency:                     in.MerchantPayoutCurrency,
		ParentOrder:                                in.ParentOrder,
		Cancellation:                               in.Cancellation,
		MccCode:                                    in.MccCode,
		OperatingCompanyId:                         in.OperatingCompanyId,
		IsHighRisk:                                 in.IsHighRisk,
		RefundAllowed:                              in.RefundAllowed,
		VatPayer:                                   in.VatPayer,
		IsProduction:                               in.IsProduction,
		TaxRate:                                    in.TaxRate,
		MerchantInfo:                               in.MerchantInfo,
		OrderCharge:                                in.OrderCharge,
		OrderChargeBeforeVat:                       in.OrderChargeBeforeVat,
		PaymentMethodTerminalId:                    in.PaymentMethodTerminalId,
		MetadataValues:                             in.MetadataValues,
		AmountBeforeVat:                            in.AmountBeforeVat,
		RoyaltyReportId:                            in.RoyaltyReportId,
		Recurring:                                  in.Recurring,
		RecurringId:                                in.RecurringId,
		ReportSummary: &MgoOrderViewReportSummary{
			Status:  in.ReportSummary.Status,
			Charge:  in.ReportSummary.Charge,
			Gross:   in.ReportSummary.Gross,
			Vat:     in.ReportSummary.Vat,
			Fees:    in.ReportSummary.Fees,
			Revenue: in.ReportSummary.Revenue,
		},
	}

	out.MerchantId = merchantOid

	if in.Project != nil && len(in.Project.Name) > 0 {
		for k, v := range in.Project.Name {
			out.Project.Name = append(out.Project.Name, &MgoMultiLang{Lang: k, Value: v})
		}
	}

	if in.Project != nil && in.Project.FirstPaymentAt != nil {
		out.Project.FirstPaymentAt, err = ptypes.Timestamp(in.Project.FirstPaymentAt)
		if err != nil {
			return nil, err
		}
	}

	if in.CreatedAt != nil {
		t, err := ptypes.Timestamp(in.CreatedAt)

		if err != nil {
			return nil, err
		}

		out.CreatedAt = t
	} else {
		out.CreatedAt = time.Now()
	}

	if in.TransactionDate != nil {
		t, err := ptypes.Timestamp(in.TransactionDate)

		if err != nil {
			return nil, err
		}

		out.TransactionDate = t
	}

	for _, v := range in.Items {
		item := &MgoOrderItem{
			Object:      v.Object,
			Sku:         v.Sku,
			Name:        v.Name,
			Description: v.Description,
			Amount:      v.Amount,
			Currency:    v.Currency,
			Images:      v.Images,
			Url:         v.Url,
			Metadata:    v.Metadata,
			Code:        v.Code,
			PlatformId:  v.PlatformId,
		}

		if len(v.Id) <= 0 {
			item.Id = primitive.NewObjectID()
		} else {
			itemOid, err := primitive.ObjectIDFromHex(v.Id)

			if err != nil {
				return nil, errors.New(billingpb.ErrorInvalidObjectId)
			}
			item.Id = itemOid
		}

		item.CreatedAt, err = ptypes.Timestamp(v.CreatedAt)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = ptypes.Timestamp(v.UpdatedAt)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}

	if len(out.Items) == 0 {
		out.Items = []*MgoOrderItem{}
	}

	if in.PaymentMethod != nil {
		paymentMethodOid, err := primitive.ObjectIDFromHex(in.PaymentMethod.Id)

		if err != nil {
			return nil, err
		}

		paymentSystemOid, err := primitive.ObjectIDFromHex(in.PaymentMethod.PaymentSystemId)

		if err != nil {
			return nil, err
		}

		out.PaymentMethod = &MgoOrderPaymentMethod{
			Id:               paymentMethodOid,
			Name:             in.PaymentMethod.Name,
			ExternalId:       in.PaymentMethod.ExternalId,
			Params:           in.PaymentMethod.Params,
			PaymentSystemId:  paymentSystemOid,
			Group:            in.PaymentMethod.Group,
			Saved:            in.PaymentMethod.Saved,
			Handler:          in.PaymentMethod.Handler,
			RefundAllowed:    in.PaymentMethod.RefundAllowed,
			RecurringAllowed: in.PaymentMethod.RecurringAllowed,
		}

		if in.Refund != nil {
			out.Refund = &MgoOrderNotificationRefund{
				Amount:        in.Refund.Amount,
				Currency:      in.Refund.Currency,
				Reason:        in.Refund.Reason,
				Code:          in.Refund.Code,
				ReceiptNumber: in.Refund.ReceiptNumber,
				ReceiptUrl:    in.Refund.ReceiptUrl,
			}
		}

		if in.PaymentMethod.Card != nil {
			out.PaymentMethod.Card = in.PaymentMethod.Card
		}
		if in.PaymentMethod.Wallet != nil {
			out.PaymentMethod.Wallet = in.PaymentMethod.Wallet
		}
		if in.PaymentMethod.CryptoCurrency != nil {
			out.PaymentMethod.CryptoCurrency = in.PaymentMethod.CryptoCurrency
		}
	}

	return out, nil
}

func (o *orderViewPrivateMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	decoded := obj.(*MgoOrderViewPrivate)
	m := &billingpb.OrderViewPrivate{}

	m.Id = decoded.Id.Hex()
	m.Uuid = decoded.Uuid
	m.TotalPaymentAmount = decoded.TotalPaymentAmount
	m.Currency = decoded.Currency
	m.Project = getOrderProject(decoded.Project)
	m.Transaction = decoded.Transaction
	m.PaymentMethod = getPaymentMethodOrder(decoded.PaymentMethod)
	m.CountryCode = decoded.CountryCode
	m.MerchantId = decoded.MerchantId.Hex()
	m.Locale = decoded.Locale
	m.Status = decoded.Status
	m.User = decoded.User
	m.BillingAddress = decoded.BillingAddress
	m.Type = decoded.Type
	m.Issuer = decoded.Issuer
	m.MerchantPayoutCurrency = decoded.MerchantPayoutCurrency
	m.IsVatDeduction = decoded.IsVatDeduction
	m.ParentOrder = decoded.ParentOrder
	m.Cancellation = decoded.Cancellation

	if decoded.Refund != nil {
		m.Refund = &billingpb.OrderNotificationRefund{
			Amount:        decoded.Refund.Amount,
			Currency:      decoded.Refund.Currency,
			Reason:        decoded.Refund.Reason,
			Code:          decoded.Refund.Code,
			ReceiptNumber: decoded.Refund.ReceiptNumber,
			ReceiptUrl:    decoded.Refund.ReceiptUrl,
		}
	}

	m.PaymentGrossRevenueLocal = getOrderViewMoney(decoded.PaymentGrossRevenueLocal)
	m.PaymentGrossRevenueOrigin = getOrderViewMoney(decoded.PaymentGrossRevenueOrigin)
	m.PaymentGrossRevenue = getOrderViewMoney(decoded.PaymentGrossRevenue)
	m.PaymentTaxFee = getOrderViewMoney(decoded.PaymentTaxFee)
	m.PaymentTaxFeeLocal = getOrderViewMoney(decoded.PaymentTaxFeeLocal)
	m.PaymentTaxFeeOrigin = getOrderViewMoney(decoded.PaymentTaxFeeOrigin)
	m.PaymentTaxFeeCurrencyExchangeFee = getOrderViewMoney(decoded.PaymentTaxFeeCurrencyExchangeFee)
	m.PaymentTaxFeeTotal = getOrderViewMoney(decoded.PaymentTaxFeeTotal)
	m.PaymentGrossRevenueFx = getOrderViewMoney(decoded.PaymentGrossRevenueFx)
	m.PaymentGrossRevenueFxTaxFee = getOrderViewMoney(decoded.PaymentGrossRevenueFxTaxFee)
	m.PaymentGrossRevenueFxProfit = getOrderViewMoney(decoded.PaymentGrossRevenueFxProfit)
	m.GrossRevenue = getOrderViewMoney(decoded.GrossRevenue)
	m.TaxFee = getOrderViewMoney(decoded.TaxFee)
	m.TaxFeeCurrencyExchangeFee = getOrderViewMoney(decoded.TaxFeeCurrencyExchangeFee)
	m.TaxFeeTotal = getOrderViewMoney(decoded.TaxFeeTotal)
	m.MethodFeeTotal = getOrderViewMoney(decoded.MethodFeeTotal)
	m.MethodFeeTariff = getOrderViewMoney(decoded.MethodFeeTariff)
	m.PaysuperMethodFeeTariffSelfCost = getOrderViewMoney(decoded.PaysuperMethodFeeTariffSelfCost)
	m.PaysuperMethodFeeProfit = getOrderViewMoney(decoded.PaysuperMethodFeeProfit)
	m.MethodFixedFeeTariff = getOrderViewMoney(decoded.MethodFixedFeeTariff)
	m.PaysuperMethodFixedFeeTariffFxProfit = getOrderViewMoney(decoded.PaysuperMethodFixedFeeTariffFxProfit)
	m.PaysuperMethodFixedFeeTariffSelfCost = getOrderViewMoney(decoded.PaysuperMethodFixedFeeTariffSelfCost)
	m.PaysuperMethodFixedFeeTariffTotalProfit = getOrderViewMoney(decoded.PaysuperMethodFixedFeeTariffTotalProfit)
	m.PaysuperFixedFee = getOrderViewMoney(decoded.PaysuperFixedFee)
	m.PaysuperFixedFeeFxProfit = getOrderViewMoney(decoded.PaysuperFixedFeeFxProfit)
	m.FeesTotal = getOrderViewMoney(decoded.FeesTotal)
	m.FeesTotalLocal = getOrderViewMoney(decoded.FeesTotalLocal)
	m.NetRevenue = getOrderViewMoney(decoded.NetRevenue)
	m.PaysuperMethodTotalProfit = getOrderViewMoney(decoded.PaysuperMethodTotalProfit)
	m.PaysuperTotalProfit = getOrderViewMoney(decoded.PaysuperTotalProfit)
	m.PaymentRefundGrossRevenueLocal = getOrderViewMoney(decoded.PaymentRefundGrossRevenueLocal)
	m.PaymentRefundGrossRevenueOrigin = getOrderViewMoney(decoded.PaymentRefundGrossRevenueOrigin)
	m.PaymentRefundGrossRevenue = getOrderViewMoney(decoded.PaymentRefundGrossRevenue)
	m.PaymentRefundTaxFee = getOrderViewMoney(decoded.PaymentRefundTaxFee)
	m.PaymentRefundTaxFeeLocal = getOrderViewMoney(decoded.PaymentRefundTaxFeeLocal)
	m.PaymentRefundTaxFeeOrigin = getOrderViewMoney(decoded.PaymentRefundTaxFeeOrigin)
	m.PaymentRefundFeeTariff = getOrderViewMoney(decoded.PaymentRefundFeeTariff)
	m.MethodRefundFixedFeeTariff = getOrderViewMoney(decoded.MethodRefundFixedFeeTariff)
	m.RefundGrossRevenue = getOrderViewMoney(decoded.RefundGrossRevenue)
	m.RefundGrossRevenueFx = getOrderViewMoney(decoded.RefundGrossRevenueFx)
	m.MethodRefundFeeTariff = getOrderViewMoney(decoded.MethodRefundFeeTariff)
	m.PaysuperMethodRefundFeeTariffProfit = getOrderViewMoney(decoded.PaysuperMethodRefundFeeTariffProfit)
	m.PaysuperMethodRefundFixedFeeTariffSelfCost = getOrderViewMoney(decoded.PaysuperMethodRefundFixedFeeTariffSelfCost)
	m.MerchantRefundFixedFeeTariff = getOrderViewMoney(decoded.MerchantRefundFixedFeeTariff)
	m.PaysuperMethodRefundFixedFeeTariffProfit = getOrderViewMoney(decoded.PaysuperMethodRefundFixedFeeTariffProfit)
	m.RefundTaxFee = getOrderViewMoney(decoded.RefundTaxFee)
	m.RefundTaxFeeCurrencyExchangeFee = getOrderViewMoney(decoded.RefundTaxFeeCurrencyExchangeFee)
	m.PaysuperRefundTaxFeeCurrencyExchangeFee = getOrderViewMoney(decoded.PaysuperRefundTaxFeeCurrencyExchangeFee)
	m.RefundTaxFeeTotal = getOrderViewMoney(decoded.RefundTaxFeeTotal)
	m.RefundReverseRevenue = getOrderViewMoney(decoded.RefundReverseRevenue)
	m.RefundFeesTotal = getOrderViewMoney(decoded.RefundFeesTotal)
	m.RefundFeesTotalLocal = getOrderViewMoney(decoded.RefundFeesTotalLocal)
	m.PaysuperRefundTotalProfit = getOrderViewMoney(decoded.PaysuperRefundTotalProfit)
	m.Items = getOrderViewItems(decoded.Items)
	m.MccCode = decoded.MccCode
	m.OperatingCompanyId = decoded.OperatingCompanyId
	m.IsHighRisk = decoded.IsHighRisk
	m.RefundAllowed = decoded.RefundAllowed
	m.VatPayer = decoded.VatPayer
	m.IsProduction = decoded.IsProduction
	m.TaxRate = decoded.TaxRate
	m.MerchantInfo = decoded.MerchantInfo
	m.OrderCharge = decoded.OrderCharge
	m.OrderChargeBeforeVat = decoded.OrderChargeBeforeVat
	m.PaymentMethodTerminalId = decoded.PaymentMethodTerminalId
	m.Recurring = decoded.Recurring
	m.RecurringId = decoded.RecurringId
	m.ReportSummary = getOrderViewReportSummary(decoded.ReportSummary)

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)
	if err != nil {
		return nil, err
	}

	m.TransactionDate, err = ptypes.TimestampProto(decoded.TransactionDate)
	if err != nil {
		return nil, err
	}

	return m, nil
}
