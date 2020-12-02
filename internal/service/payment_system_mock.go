package service

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/internal/payment_system"
	"github.com/paysuper/paysuper-billing-server/pkg"
	errors2 "github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type PaymentSystemMockOk struct{}
type PaymentSystemMockError struct{}

func NewPaymentSystemMockOk() payment_system.PaymentSystemInterface {
	return &PaymentSystemMockOk{}
}

func NewPaymentSystemMockError() payment_system.PaymentSystemInterface {
	return &PaymentSystemMockError{}
}

func NewCardPayMock() payment_system.PaymentSystemInterface {
	cpMock := &mocks.PaymentSystemInterface{}
	cpMock.On("CreatePayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			func(order *billingpb.Order, successUrl, failUrl string, requisites map[string]string) string {
				order.PrivateStatus = recurringpb.OrderStatusPaymentSystemCreate
				return "http://localhost"
			},
			nil,
		)
	cpMock.On("ProcessPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			func(order *billingpb.Order, message proto.Message, raw, signature string) error {
				req := message.(*billingpb.CardPayPaymentCallback)

				t, _ := time.Parse(payment_system.CardPayDateFormat, req.CallbackTime)
				ts, _ := ptypes.TimestampProto(t)

				order.PaymentMethodTxnParams = map[string]string{

					"emission_country": "US",
					"token":            "",
					"rrn":              "",
					"is_3ds":           "1",
					"pan":              req.CardAccount.MaskedPan,
					"card_holder":      "UNIT TEST",
				}
				order.PrivateStatus = recurringpb.OrderStatusPaymentSystemComplete
				order.Transaction = req.GetId()
				order.PaymentMethodOrderClosedAt = ts

				if req.GetAmount() == 123 {
					return errors2.NewBillingServerResponseError(pkg.StatusErrorValidation, payment_system.PaymentSystemErrorRequestAmountOrCurrencyIsInvalid)
				}

				return nil
			},
			nil,
		)
	cpMock.On("IsRecurringCallback", mock.Anything).Return(false)
	cpMock.On("GetRecurringId", mock.Anything).Return("0987654321")
	cpMock.On("CreateRefund", mock.Anything, mock.Anything).
		Return(
			func(order *billingpb.Order, refund *billingpb.Refund) error {
				refund.Status = pkg.RefundStatusInProgress
				refund.ExternalId = "0987654321"
				return nil
			},
			nil,
		)
	cpMock.On("ProcessRefund", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			func(order *billingpb.Order, refund *billingpb.Refund, message proto.Message, raw, signature string) error {
				req := message.(*billingpb.CardPayRefundCallback)

				t, _ := time.Parse(payment_system.CardPayDateFormat, req.CallbackTime)
				ts, _ := ptypes.TimestampProto(t)

				if refund.Reason == "unit test decline" {
					refund.Status = pkg.RefundStatusPaymentSystemDeclined
				} else {
					refund.Status = pkg.RefundStatusCompleted
				}

				refund.ExternalId = "0987654321"
				refund.UpdatedAt = ts

				order.PaymentMethodOrderClosedAt = ts
				return nil
			},
			nil,
		)
	cpMock.On("CreateRecurringSubscription", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			func(order *billingpb.Order, subscription *recurringpb.Subscription, successUrl, failUrl string, requisites map[string]string) string {
				return "http://localhost"
			},
			nil,
		)
	cpMock.On("IsSubscriptionCallback", mock.Anything).Return(false, nil)
	cpMock.On("DeleteRecurringSubscription", mock.Anything, mock.Anything).
		Return(nil, nil)
	cpMock.On("CanSaveCard", mock.Anything).Return(false)
	return cpMock
}

func (m *PaymentSystemMockOk) CreatePayment(_ *billingpb.Order, _, _ string, _ map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockOk) ProcessPayment(_ *billingpb.Order, _ proto.Message, _, _ string) error {
	return nil
}

func (m *PaymentSystemMockOk) IsRecurringCallback(_ proto.Message) bool {
	return false
}

func (m *PaymentSystemMockOk) GetRecurringId(_ proto.Message) string {
	return ""
}

func (m *PaymentSystemMockOk) CreateRefund(_ *billingpb.Order, refund *billingpb.Refund) error {
	refund.Status = pkg.RefundStatusInProgress
	refund.ExternalId = primitive.NewObjectID().Hex()

	return nil
}

func (m *PaymentSystemMockOk) ProcessRefund(_ *billingpb.Order, refund *billingpb.Refund, _ proto.Message, _, _ string) error {
	refund.Status = pkg.RefundStatusCompleted
	refund.ExternalId = primitive.NewObjectID().Hex()

	return nil
}

func (m *PaymentSystemMockOk) CreateRecurringSubscription(_ *billingpb.Order, _ *recurringpb.Subscription, _, _ string, _ map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockOk) IsSubscriptionCallback(_ proto.Message) bool {
	return false
}

func (m *PaymentSystemMockOk) DeleteRecurringSubscription(_ *billingpb.Order, _ *recurringpb.Subscription) error {
	return nil
}

func (m *PaymentSystemMockOk) CanSaveCard(_ proto.Message) bool {
	return false
}

func (m *PaymentSystemMockError) CreatePayment(_ *billingpb.Order, _, _ string, _ map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockError) ProcessPayment(_ *billingpb.Order, _ proto.Message, _, _ string) error {
	return nil
}

func (m *PaymentSystemMockError) IsRecurringCallback(_ proto.Message) bool {
	return false
}

func (m *PaymentSystemMockError) GetRecurringId(_ proto.Message) string {
	return ""
}

func (m *PaymentSystemMockError) CreateRefund(_ *billingpb.Order, refund *billingpb.Refund) error {
	refund.Status = pkg.RefundStatusRejected
	return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
}

func (m *PaymentSystemMockError) ProcessRefund(_ *billingpb.Order, refund *billingpb.Refund, _ proto.Message, _, _ string) error {
	refund.Status = pkg.RefundStatusRejected
	return errors2.NewBillingServerResponseError(billingpb.ResponseStatusBadData, payment_system.PaymentSystemErrorRefundRequestAmountOrCurrencyIsInvalid)
}

func (m *PaymentSystemMockError) CreateRecurringSubscription(_ *billingpb.Order, _ *recurringpb.Subscription, _, _ string, _ map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockError) IsSubscriptionCallback(_ proto.Message) bool {
	return false
}

func (m *PaymentSystemMockError) DeleteRecurringSubscription(_ *billingpb.Order, _ *recurringpb.Subscription) error {
	return nil
}

func (m *PaymentSystemMockError) CanSaveCard(_ proto.Message) bool {
	return false
}
