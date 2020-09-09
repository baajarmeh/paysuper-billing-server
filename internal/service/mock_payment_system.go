package service

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type PaymentSystemMockOk struct{}
type PaymentSystemMockError struct{}

func NewPaymentSystemMockOk() Gate {
	return &PaymentSystemMockOk{}
}

func NewPaymentSystemMockError() Gate {
	return &PaymentSystemMockError{}
}

func NewCardPayMock() Gate {
	cpMock := &mocks.PaymentSystem{}
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

				t, _ := time.Parse(cardPayDateFormat, req.CallbackTime)
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
					return newBillingServerResponseError(pkg.StatusErrorValidation, paymentSystemErrorRequestAmountOrCurrencyIsInvalid)
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

				t, _ := time.Parse(cardPayDateFormat, req.CallbackTime)
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
	return cpMock
}

func (m *PaymentSystemMockOk) CreatePayment(order *billingpb.Order, successUrl, failUrl string, requisites map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockOk) ProcessPayment(order *billingpb.Order, message proto.Message, raw, signature string) error {
	return nil
}

func (m *PaymentSystemMockOk) IsRecurringCallback(request proto.Message) bool {
	return false
}

func (m *PaymentSystemMockOk) GetRecurringId(request proto.Message) string {
	return ""
}

func (m *PaymentSystemMockOk) CreateRefund(order *billingpb.Order, refund *billingpb.Refund) error {
	refund.Status = pkg.RefundStatusInProgress
	refund.ExternalId = primitive.NewObjectID().Hex()

	return nil
}

func (m *PaymentSystemMockOk) ProcessRefund(order *billingpb.Order, refund *billingpb.Refund, message proto.Message, raw, signature string) error {
	refund.Status = pkg.RefundStatusCompleted
	refund.ExternalId = primitive.NewObjectID().Hex()

	return nil
}

func (m *PaymentSystemMockError) CreatePayment(order *billingpb.Order, successUrl, failUrl string, requisites map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockError) ProcessPayment(order *billingpb.Order, message proto.Message, raw, signature string) error {
	return nil
}

func (m *PaymentSystemMockError) IsRecurringCallback(request proto.Message) bool {
	return false
}

func (m *PaymentSystemMockError) GetRecurringId(request proto.Message) string {
	return ""
}

func (m *PaymentSystemMockError) CreateRefund(order *billingpb.Order, refund *billingpb.Refund) error {
	refund.Status = pkg.RefundStatusRejected
	return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
}

func (m *PaymentSystemMockError) ProcessRefund(order *billingpb.Order, refund *billingpb.Refund, message proto.Message, raw, signature string) error {
	refund.Status = pkg.RefundStatusRejected
	return newBillingServerResponseError(billingpb.ResponseStatusBadData, paymentSystemErrorRefundRequestAmountOrCurrencyIsInvalid)
}
