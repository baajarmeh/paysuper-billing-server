package repository

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
	"time"
)

type OrderViewTestSuite struct {
	suite.Suite
	db              mongodb.SourceInterface
	orderRepository OrderRepositoryInterface
	repository      OrderViewRepositoryInterface
	logObserver     *zap.Logger
	zapRecorder     *observer.ObservedLogs
}

func Test_OrderView(t *testing.T) {
	suite.Run(t, new(OrderViewTestSuite))
}

func (suite *OrderViewTestSuite) SetupTest() {
	_, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)
	zap.ReplaceGlobals(suite.logObserver)

	suite.db, err = mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	suite.orderRepository = NewOrderRepository(suite.db)
	suite.repository = NewOrderViewRepository(suite.db)
}

func (suite *OrderViewTestSuite) TearDownTest() {
	if err := suite.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	if err := suite.db.Close(); err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *OrderViewTestSuite) TestFindForRecurringSubscriptionsByUserId() {
	order1 := suite.getOrderTemplate()
	err := suite.orderRepository.Insert(context.TODO(), order1)
	assert.NoError(suite.T(), err)

	order2 := suite.getOrderTemplate()
	err = suite.orderRepository.Insert(context.TODO(), order2)
	assert.NoError(suite.T(), err)

	err = suite.orderRepository.UpdateOrderView(context.TODO(), []string{order1.Id, order2.Id})
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.FindForRecurringSubscriptions(context.TODO(), order2.User.Id, "", "", "", "", nil, nil, 10, 0)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order2.Id, orders[0].Id)

	count, err := suite.repository.CountForRecurringSubscriptions(context.TODO(), order2.User.Id, "", "", "", "", nil, nil)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

func (suite *OrderViewTestSuite) TestFindForRecurringSubscriptionsByMerchantId() {
	order1 := suite.getOrderTemplate()
	err := suite.orderRepository.Insert(context.TODO(), order1)
	assert.NoError(suite.T(), err)

	order2 := suite.getOrderTemplate()
	err = suite.orderRepository.Insert(context.TODO(), order2)
	assert.NoError(suite.T(), err)

	err = suite.orderRepository.UpdateOrderView(context.TODO(), []string{order1.Id, order2.Id})
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.FindForRecurringSubscriptions(context.TODO(), "", order2.Project.MerchantId, "", "", "", nil, nil, 10, 0)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order2.Id, orders[0].Id)

	count, err := suite.repository.CountForRecurringSubscriptions(context.TODO(), "", order2.Project.MerchantId, "", "", "", nil, nil)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

func (suite *OrderViewTestSuite) TestFindForRecurringSubscriptionsByProjectId() {
	order1 := suite.getOrderTemplate()
	err := suite.orderRepository.Insert(context.TODO(), order1)
	assert.NoError(suite.T(), err)

	order2 := suite.getOrderTemplate()
	err = suite.orderRepository.Insert(context.TODO(), order2)
	assert.NoError(suite.T(), err)

	err = suite.orderRepository.UpdateOrderView(context.TODO(), []string{order1.Id, order2.Id})
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.FindForRecurringSubscriptions(context.TODO(), "", "", order2.Project.Id, "", "", nil, nil, 10, 0)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order2.Id, orders[0].Id)

	count, err := suite.repository.CountForRecurringSubscriptions(context.TODO(), "", "", order2.Project.Id, "", "", nil, nil)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

func (suite *OrderViewTestSuite) TestFindForRecurringSubscriptionsBySubscriptionId() {
	order1 := suite.getOrderTemplate()
	err := suite.orderRepository.Insert(context.TODO(), order1)
	assert.NoError(suite.T(), err)

	order2 := suite.getOrderTemplate()
	err = suite.orderRepository.Insert(context.TODO(), order2)
	assert.NoError(suite.T(), err)

	err = suite.orderRepository.UpdateOrderView(context.TODO(), []string{order1.Id, order2.Id})
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.FindForRecurringSubscriptions(context.TODO(), "", "", "", order2.RecurringSubscriptionId, "", nil, nil, 10, 0)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order2.Id, orders[0].Id)

	count, err := suite.repository.CountForRecurringSubscriptions(context.TODO(), "", "", "", order2.RecurringSubscriptionId, "", nil, nil)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

func (suite *OrderViewTestSuite) TestFindForRecurringSubscriptionsByStatus() {
	order1 := suite.getOrderTemplate()
	err := suite.orderRepository.Insert(context.TODO(), order1)
	assert.NoError(suite.T(), err)

	order2 := suite.getOrderTemplate()
	order2.PrivateStatus = recurringpb.OrderStatusPaymentSystemCanceled
	err = suite.orderRepository.Insert(context.TODO(), order2)
	assert.NoError(suite.T(), err)

	err = suite.orderRepository.UpdateOrderView(context.TODO(), []string{order1.Id, order2.Id})
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.FindForRecurringSubscriptions(context.TODO(), "", "", "", "", recurringpb.OrderPublicStatusCanceled, nil, nil, 10, 0)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order2.Id, orders[0].Id)

	count, err := suite.repository.CountForRecurringSubscriptions(context.TODO(), "", "", "", "", recurringpb.OrderPublicStatusCanceled, nil, nil)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

func (suite *OrderViewTestSuite) TestFindForRecurringSubscriptionsByDates() {
	now := time.Now().UTC()
	dateFrom := now.AddDate(0, 0, -2)
	dateTo := now.AddDate(0, 0, -1)

	order1 := suite.getOrderTemplate()
	err := suite.orderRepository.Insert(context.TODO(), order1)
	assert.NoError(suite.T(), err)

	order2 := suite.getOrderTemplate()
	order2.CreatedAt, _ = ptypes.TimestampProto(dateFrom)
	err = suite.orderRepository.Insert(context.TODO(), order2)
	assert.NoError(suite.T(), err)

	err = suite.orderRepository.UpdateOrderView(context.TODO(), []string{order1.Id, order2.Id})
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.FindForRecurringSubscriptions(context.TODO(), "", "", "", "", "", &dateFrom, &dateTo, 10, 0)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order2.Id, orders[0].Id)

	count, err := suite.repository.CountForRecurringSubscriptions(context.TODO(), "", "", "", "", "", &dateFrom, &dateTo)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

func (suite *OrderViewTestSuite) TestFindForRecurringSubscriptionsLimitOffset() {
	order1 := suite.getOrderTemplate()
	err := suite.orderRepository.Insert(context.TODO(), order1)
	assert.NoError(suite.T(), err)

	order2 := suite.getOrderTemplate()
	order2.Project.MerchantId = order1.Project.MerchantId
	err = suite.orderRepository.Insert(context.TODO(), order2)
	assert.NoError(suite.T(), err)

	err = suite.orderRepository.UpdateOrderView(context.TODO(), []string{order1.Id, order2.Id})
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.FindForRecurringSubscriptions(context.TODO(), "", order1.Project.MerchantId, "", "", "", nil, nil, 1, 0)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order1.Id, orders[0].Id)

	orders, err = suite.repository.FindForRecurringSubscriptions(context.TODO(), "", order1.Project.MerchantId, "", "", "", nil, nil, 1, 1)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), orders)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order2.Id, orders[0].Id)
}

func (suite *OrderViewTestSuite) getOrderTemplate() *billingpb.Order {

	return &billingpb.Order{
		Id: primitive.NewObjectID().Hex(),
		Project: &billingpb.ProjectOrder{
			Id:                      primitive.NewObjectID().Hex(),
			MerchantId:              primitive.NewObjectID().Hex(),
			Status:                  1,
			Name:                    map[string]string{"en": "string"},
			SecretKey:               "SecretKey",
			CallbackProtocol:        "CallbackProtocol",
			MerchantRoyaltyCurrency: "MerchantRoyaltyCurrency",
			NotifyEmails:            []string{"email"},
			SendNotifyEmail:         true,
			UrlCancelPayment:        "UrlCancelPayment",
			UrlChargebackPayment:    "UrlChargebackPayment",
			UrlCheckAccount:         "UrlCheckAccount",
			UrlFail:                 "UrlFail",
			UrlFraudPayment:         "UrlFraudPayment",
			UrlProcessPayment:       "UrlProcessPayment",
			UrlRefundPayment:        "UrlRefundPayment",
			UrlSuccess:              "UrlSuccess",
			FirstPaymentAt:          &timestamp.Timestamp{Seconds: 100},
		},
		Tax: &billingpb.OrderTax{
			Rate: 1,
		},
		User: &billingpb.OrderUser{
			Id: primitive.NewObjectID().Hex(),
		},
		Uuid:                        "Uuid",
		Status:                      "processed",
		Currency:                    "Currency",
		Type:                        "Type",
		OperatingCompanyId:          primitive.NewObjectID().Hex(),
		PlatformId:                  "PlatformId",
		ReceiptId:                   "ReceiptId",
		CountryCode:                 "",
		Products:                    []string{primitive.NewObjectID().Hex()},
		IsVatDeduction:              true,
		TotalPaymentAmount:          1,
		Transaction:                 "Transaction",
		Object:                      "order",
		AgreementAccepted:           true,
		AgreementVersion:            "AgreementVersion",
		BillingCountryChangedByUser: true,
		Canceled:                    false,
		ChargeAmount:                2,
		ChargeCurrency:              "ChargeCurrency",
		Description:                 "Description",
		IsBuyForVirtualCurrency:     true,
		IsCurrencyPredefined:        false,
		IsHighRisk:                  true,
		IsIpCountryMismatchBin:      true,
		IsJsonRequest:               true,
		IsKeyProductNotified:        true,
		IsRefundAllowed:             true,
		IsNotificationsSent:         map[string]bool{"string": true},
		Keys:                        []string{"string"},
		MccCode:                     "MccCode",
		NotifySale:                  true,
		NotifySaleEmail:             "NotifySaleEmail",
		PaymentIpCountry:            "PaymentIpCountry",
		OrderAmount:                 3,
		PaymentMethodPayerAccount:   "pm_payer_account",
		PaymentMethodTxnParams:      map[string]string{"string": "a"},
		Metadata:                    map[string]string{"string": "b"},
		PaymentRequisites:           map[string]string{"string": "c"},
		PrivateMetadata:             map[string]string{"string": "d"},
		PrivateStatus:               4,
		ProductType:                 "ProductType",
		ProjectParams:               map[string]string{"string": "e"},
		ReceiptEmail:                "",
		ReceiptPhone:                "",
		ReceiptNumber:               "Phone",
		ReceiptUrl:                  "ReceiptUrl",
		Refunded:                    false,
		UserAddressDataRequired:     true,
		VatPayer:                    "VatPayer",
		VirtualCurrencyAmount:       0,
		Items:                       []*billingpb.OrderItem{},
		UpdatedAt:                   &timestamp.Timestamp{Seconds: 100},
		CreatedAt:                   &timestamp.Timestamp{Seconds: 100},
		CanceledAt:                  &timestamp.Timestamp{Seconds: 100},
		ExpireDateToFormInput:       &timestamp.Timestamp{Seconds: 100},
		ParentPaymentAt:             &timestamp.Timestamp{Seconds: 100},
		PaymentMethodOrderClosedAt:  &timestamp.Timestamp{Seconds: 100},
		ProjectLastRequestedAt:      &timestamp.Timestamp{Seconds: 100},
		RefundedAt:                  &timestamp.Timestamp{Seconds: 100},
		RecurringSubscriptionId:     primitive.NewObjectID().Hex(),
	}
}
