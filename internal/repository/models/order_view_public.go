package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type orderViewPublicMapper struct {
}

type MgoOrderViewPublic struct {
	Id                                      primitive.ObjectID                       `bson:"_id" faker:"objectId"`
	Uuid                                    string                                   `bson:"uuid"`
	TotalPaymentAmount                      float64                                  `bson:"total_payment_amount"`
	Currency                                string                                   `bson:"currency"`
	Project                                 *MgoOrderProject                         `bson:"project"`
	CreatedAt                               time.Time                                `bson:"created_at"`
	Transaction                             string                                   `bson:"pm_order_id"`
	PaymentMethod                           *MgoOrderPaymentMethod                   `bson:"payment_method"`
	CountryCode                             string                                   `bson:"country_code"`
	MerchantId                              primitive.ObjectID                       `bson:"merchant_id" faker:"objectId"`
	Locale                                  string                                   `bson:"locale"`
	Status                                  string                                   `bson:"status"`
	TransactionDate                         time.Time                                `bson:"pm_order_close_date"`
	User                                    *billingpb.OrderUser                     `bson:"user"`
	BillingAddress                          *billingpb.OrderBillingAddress           `bson:"billing_address"`
	Type                                    string                                   `bson:"type"`
	IsVatDeduction                          bool                                     `bson:"is_vat_deduction"`
	GrossRevenue                            *billingpb.OrderViewMoney                `bson:"gross_revenue"`
	TaxFee                                  *billingpb.OrderViewMoney                `bson:"tax_fee"`
	TaxFeeCurrencyExchangeFee               *billingpb.OrderViewMoney                `bson:"tax_fee_currency_exchange_fee"`
	TaxFeeTotal                             *billingpb.OrderViewMoney                `bson:"tax_fee_total"`
	MethodFeeTotal                          *billingpb.OrderViewMoney                `bson:"method_fee_total"`
	MethodFeeTariff                         *billingpb.OrderViewMoney                `bson:"method_fee_tariff"`
	MethodFixedFeeTariff                    *billingpb.OrderViewMoney                `bson:"method_fixed_fee_tariff"`
	PaysuperFixedFee                        *billingpb.OrderViewMoney                `bson:"paysuper_fixed_fee"`
	FeesTotal                               *billingpb.OrderViewMoney                `bson:"fees_total"`
	FeesTotalLocal                          *billingpb.OrderViewMoney                `bson:"fees_total_local"`
	NetRevenue                              *billingpb.OrderViewMoney                `bson:"net_revenue"`
	RefundGrossRevenue                      *billingpb.OrderViewMoney                `bson:"refund_gross_revenue"`
	MethodRefundFeeTariff                   *billingpb.OrderViewMoney                `bson:"method_refund_fee_tariff"`
	MerchantRefundFixedFeeTariff            *billingpb.OrderViewMoney                `bson:"merchant_refund_fixed_fee_tariff"`
	RefundTaxFee                            *billingpb.OrderViewMoney                `bson:"refund_tax_fee"`
	RefundTaxFeeCurrencyExchangeFee         *billingpb.OrderViewMoney                `bson:"refund_tax_fee_currency_exchange_fee"`
	PaysuperRefundTaxFeeCurrencyExchangeFee *billingpb.OrderViewMoney                `bson:"paysuper_refund_tax_fee_currency_exchange_fee"`
	RefundReverseRevenue                    *billingpb.OrderViewMoney                `bson:"refund_reverse_revenue"`
	RefundFeesTotal                         *billingpb.OrderViewMoney                `bson:"refund_fees_total"`
	RefundFeesTotalLocal                    *billingpb.OrderViewMoney                `bson:"refund_fees_total_local"`
	Issuer                                  *billingpb.OrderIssuer                   `bson:"issuer"`
	Items                                   []*MgoOrderItem                          `bson:"items"`
	MerchantPayoutCurrency                  string                                   `bson:"merchant_payout_currency"`
	ParentOrder                             *billingpb.ParentOrder                   `bson:"parent_order"`
	Refund                                  *MgoOrderNotificationRefund              `bson:"refund"`
	Cancellation                            *billingpb.OrderNotificationCancellation `bson:"cancellation"`
	OperatingCompanyId                      string                                   `bson:"operating_company_id"`
	RefundAllowed                           bool                                     `bson:"refund_allowed"`
	OrderCharge                             *billingpb.OrderViewMoney                `bson:"order_charge"`
	OrderChargeBeforeVat                    *billingpb.OrderViewMoney                `bson:"order_charge_before_vat"`
	PaymentIpCountry                        string                                   `bson:"payment_ip_country"`
	IsIpCountryMismatchBin                  bool                                     `bson:"is_ip_country_mismatch_bin"`
	BillingCountryChangedByUser             bool                                     `bson:"billing_country_changed_by_user"`
	VatPayer                                string                                   `bson:"vat_payer"`
	IsProduction                            bool                                     `bson:"is_production"`
	MerchantInfo                            *billingpb.OrderViewMerchantInfo         `bson:"merchant_info"`
	Recurring                               bool                                     `bson:"recurring"`
	RecurringId                             string                                   `bson:"recurring_id"`
	PaymentGrossRevenue                     *billingpb.OrderViewMoney                `bson:"payment_gross_revenue"`
	PaymentRefundGrossRevenue               *billingpb.OrderViewMoney                `bson:"payment_refund_gross_revenue"`
	RefundTaxFeeTotal                       *billingpb.OrderViewMoney                `bson:"refund_tax_fee_total"`
	ReportSummary                           *MgoOrderViewReportSummary               `bson:"report_summary" json:"report_summary"`
}

func NewOrderViewPublicMapper() Mapper {
	return &orderViewPublicMapper{}
}

func (o *orderViewPublicMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	panic("not supported")
}

func (o *orderViewPublicMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	decoded := obj.(*MgoOrderViewPublic)
	m := &billingpb.OrderViewPublic{}

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

	m.GrossRevenue = getOrderViewMoney(decoded.GrossRevenue)
	m.TaxFee = getOrderViewMoney(decoded.TaxFee)
	m.TaxFeeCurrencyExchangeFee = getOrderViewMoney(decoded.TaxFeeCurrencyExchangeFee)
	m.TaxFeeTotal = getOrderViewMoney(decoded.TaxFeeTotal)
	m.MethodFeeTotal = getOrderViewMoney(decoded.MethodFeeTotal)
	m.MethodFeeTariff = getOrderViewMoney(decoded.MethodFeeTariff)
	m.MethodFixedFeeTariff = getOrderViewMoney(decoded.MethodFixedFeeTariff)
	m.PaysuperFixedFee = getOrderViewMoney(decoded.PaysuperFixedFee)
	m.FeesTotal = getOrderViewMoney(decoded.FeesTotal)
	m.FeesTotalLocal = getOrderViewMoney(decoded.FeesTotalLocal)
	m.NetRevenue = getOrderViewMoney(decoded.NetRevenue)
	m.RefundGrossRevenue = getOrderViewMoney(decoded.RefundGrossRevenue)
	m.MethodRefundFeeTariff = getOrderViewMoney(decoded.MethodRefundFeeTariff)
	m.MerchantRefundFixedFeeTariff = getOrderViewMoney(decoded.MerchantRefundFixedFeeTariff)
	m.RefundTaxFee = getOrderViewMoney(decoded.RefundTaxFee)
	m.RefundTaxFeeCurrencyExchangeFee = getOrderViewMoney(decoded.RefundTaxFeeCurrencyExchangeFee)
	m.PaysuperRefundTaxFeeCurrencyExchangeFee = getOrderViewMoney(decoded.PaysuperRefundTaxFeeCurrencyExchangeFee)
	m.RefundReverseRevenue = getOrderViewMoney(decoded.RefundReverseRevenue)
	m.RefundFeesTotal = getOrderViewMoney(decoded.RefundFeesTotal)
	m.RefundFeesTotalLocal = getOrderViewMoney(decoded.RefundFeesTotalLocal)
	m.Items = getOrderViewItems(decoded.Items)
	m.OperatingCompanyId = decoded.OperatingCompanyId
	m.RefundAllowed = decoded.RefundAllowed
	m.OrderCharge = decoded.OrderCharge
	m.PaymentIpCountry = decoded.PaymentIpCountry
	m.IsIpCountryMismatchBin = decoded.IsIpCountryMismatchBin
	m.BillingCountryChangedByUser = decoded.BillingCountryChangedByUser
	m.VatPayer = decoded.VatPayer
	m.IsProduction = decoded.IsProduction
	m.MerchantInfo = decoded.MerchantInfo
	m.Recurring = decoded.Recurring
	m.RecurringId = decoded.RecurringId

	m.PaymentGrossRevenue = getOrderViewMoney(decoded.PaymentGrossRevenue)
	m.PaymentRefundGrossRevenue = getOrderViewMoney(decoded.PaymentRefundGrossRevenue)
	m.RefundTaxFeeTotal = getOrderViewMoney(decoded.RefundTaxFeeTotal)

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)
	if err != nil {
		return nil, err
	}

	m.TransactionDate, err = ptypes.TimestampProto(decoded.TransactionDate)
	if err != nil {
		return nil, err
	}

	m.ReportSummary = getOrderViewReportSummary(decoded.ReportSummary)

	return m, nil
}
