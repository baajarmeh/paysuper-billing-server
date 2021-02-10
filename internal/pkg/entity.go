package pkg

import (
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type PaymentChannelCostMerchantSet struct {
	Id  string                                  `bson:"_id"`
	Set []*billingpb.PaymentChannelCostMerchant `bson:"set"`
}

type MgoPaymentChannelCostMerchantSet struct {
	Id  string                                  `bson:"_id"`
	Set []*models.MgoPaymentChannelCostMerchant `bson:"set"`
}

type PaymentChannelCostSystemSet struct {
	Id  string                                `bson:"_id"`
	Set []*billingpb.PaymentChannelCostSystem `bson:"set"`
}

type MgoPaymentChannelCostSystemSet struct {
	Id  string                                `bson:"_id"`
	Set []*models.MgoPaymentChannelCostSystem `bson:"set"`
}

type MoneyBackCostMerchantSet struct {
	Id  string                             `bson:"_id"`
	Set []*billingpb.MoneyBackCostMerchant `bson:"set"`
}

type MgoMoneyBackCostMerchantSet struct {
	Id  string                             `bson:"_id"`
	Set []*models.MgoMoneyBackCostMerchant `bson:"set"`
}

type MoneyBackCostSystemSet struct {
	Id  string                           `bson:"_id"`
	Set []*billingpb.MoneyBackCostSystem `bson:"set"`
}

type MgoMoneyBackCostSystemSet struct {
	Id  string                           `bson:"_id"`
	Set []*models.MgoMoneyBackCostSystem `bson:"set"`
}

type BinData struct {
	Id                 primitive.ObjectID `bson:"_id"`
	CardBin            int32              `bson:"card_bin"`
	CardBrand          string             `bson:"card_brand"`
	CardType           string             `bson:"card_type"`
	CardCategory       string             `bson:"card_category"`
	BankName           string             `bson:"bank_name"`
	BankCountryName    string             `bson:"bank_country_name"`
	BankCountryIsoCode string             `bson:"bank_country_code_a2"`
	BankSite           string             `bson:"bank_site"`
	BankPhone          string             `bson:"bank_phone"`
}

type ReserveQueryResItem struct {
	Type   string  `bson:"_id"`
	Amount float64 `bson:"amount"`
}

type VatReportQueryResItem struct {
	Id                             string  `bson:"_id"`
	Count                          int32   `bson:"count"`
	PaymentGrossRevenueLocal       float64 `bson:"payment_gross_revenue_local"`
	PaymentTaxFeeLocal             float64 `bson:"payment_tax_fee_local"`
	PaymentRefundGrossRevenueLocal float64 `bson:"payment_refund_gross_revenue_local"`
	PaymentRefundTaxFeeLocal       float64 `bson:"payment_refund_tax_fee_local"`
	PaymentFeesTotal               float64 `bson:"fees_total"`
	PaymentRefundFeesTotal         float64 `bson:"refund_fees_total"`
}

type TurnoverQueryResItem struct {
	Id     string  `bson:"_id"`
	Amount float64 `bson:"amount"`
}

type RoyaltyReportMerchant struct {
	Id primitive.ObjectID `bson:"_id"`
}

type RoyaltySummaryResult struct {
	Items     []*billingpb.RoyaltyReportProductSummaryItem `bson:"top"`
	Total     *billingpb.RoyaltyReportProductSummaryItem   `bson:"total"`
	OrdersIds []primitive.ObjectID                         `bson:"orders_ids"`
}

type PaylinkVisits struct {
	PaylinkId primitive.ObjectID `bson:"paylink_id"`
	Date      time.Time          `bson:"date"`
}

type GrossRevenueAndVatReports struct {
	GrossRevenue *billingpb.DashboardAmountItemWithChart `bson:"gross_revenue"`
	Vat          *billingpb.DashboardAmountItemWithChart `bson:"vat"`
}

type TotalTransactionsAndArpuReports struct {
	TotalTransactions *billingpb.DashboardMainReportTotalTransactions `bson:"total_transactions"`
	Arpu              *billingpb.DashboardAmountItemWithChart         `bson:"arpu"`
}

type BalanceQueryResult struct {
	Currency             string  `bson:"currency"`
	GrossTotalAmount     float64 `bson:"gross_total_amount"`
	TotalFees            float64 `bson:"total_fees"`
	TotalVat             float64 `bson:"total_vat"`
	PayoutAmount         float64 `bson:"payout_amount"`
	CorrectionAmount     float64 `bson:"correction_amount"`
	RollingReserveAmount float64 `bson:"rolling_reserve_total_amount"`
	FinalPayoutAmount    float64 `bson:"final_payout_amount"`
}
