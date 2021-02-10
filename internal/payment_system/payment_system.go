package payment_system

import (
	"github.com/golang/protobuf/proto"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
)

const (
	defaultHttpClientTimeout = 10
)

var (
	PaymentSystemErrorHandlerNotFound                        = errors.NewBillingServerErrorMsg("ph000001", "handler for specified payment system not found")
	paymentSystemErrorAuthenticateFailed                     = errors.NewBillingServerErrorMsg("ph000002", "authentication failed")
	paymentSystemErrorUnknownPaymentMethod                   = errors.NewBillingServerErrorMsg("ph000003", "unknown payment Method")
	paymentSystemErrorCreateRequestFailed                    = errors.NewBillingServerErrorMsg("ph000004", "order can't be create. try request later")
	PaymentSystemErrorEWalletIdentifierIsInvalid             = errors.NewBillingServerErrorMsg("ph000005", "wallet identifier is invalid")
	paymentSystemErrorRequestSignatureIsInvalid              = errors.NewBillingServerErrorMsg("ph000006", "request signature is invalid")
	paymentSystemErrorRequestTimeFieldIsInvalid              = errors.NewBillingServerErrorMsg("ph000007", "time field in request is invalid")
	paymentSystemErrorRequestRecurringIdFieldIsInvalid       = errors.NewBillingServerErrorMsg("ph000008", "recurring id field in request is invalid")
	paymentSystemErrorRequestStatusIsInvalid                 = errors.NewBillingServerErrorMsg("ph000009", "status is invalid")
	paymentSystemErrorRequestPaymentMethodIsInvalid          = errors.NewBillingServerErrorMsg("ph000010", "payment Method from request not match with value in order")
	PaymentSystemErrorRequestAmountOrCurrencyIsInvalid       = errors.NewBillingServerErrorMsg("ph000011", "amount or currency from request not match with value in order")
	PaymentSystemErrorRefundRequestAmountOrCurrencyIsInvalid = errors.NewBillingServerErrorMsg("ph000012", "amount or currency from request not match with value in refund")
	PaymentSystemErrorRequestTemporarySkipped                = errors.NewBillingServerErrorMsg("ph000013", "notification skipped with temporary status")
	paymentSystemErrorRecurringFailed                        = errors.NewBillingServerErrorMsg("ph000014", "recurring payment failed")
	paymentSystemErrorCreateRecurringPlanFailed              = errors.NewBillingServerErrorMsg("ph000015", "create recurring plan failed")
	paymentSystemErrorCreateRecurringSubscriptionFailed      = errors.NewBillingServerErrorMsg("ph000016", "create recurring subscription failed")
	paymentSystemErrorDeleteRecurringPlanFailed              = errors.NewBillingServerErrorMsg("ph000017", "delete recurring plan failed")
	paymentSystemErrorUpdateRecurringSubscriptionFailed      = errors.NewBillingServerErrorMsg("ph000018", "update recurring subscription failed")
)

type PaymentSystemInterface interface {
	CreatePayment(order *billingpb.Order, successUrl, failUrl string, requisites map[string]string) (string, error)
	ProcessPayment(order *billingpb.Order, message proto.Message, raw, signature string) error
	IsRecurringCallback(request proto.Message) bool
	CanSaveCard(request proto.Message) bool
	GetRecurringId(request proto.Message) string
	CreateRefund(order *billingpb.Order, refund *billingpb.Refund) error
	ProcessRefund(order *billingpb.Order, refund *billingpb.Refund, message proto.Message, raw, signature string) error
	CreateRecurringSubscription(order *billingpb.Order, subscription *billingpb.RecurringSubscription, successUrl, failUrl string, requisites map[string]string) (string, error)
	IsSubscriptionCallback(request proto.Message) bool
	DeleteRecurringSubscription(order *billingpb.Order, subscription *billingpb.RecurringSubscription) error
}

type PaymentSystemManagerInterface interface {
	GetGateway(name string) (PaymentSystemInterface, error)
}
