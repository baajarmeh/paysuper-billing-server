package repository

import (
	"context"
	"errors"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type OrderTestSuite struct {
	suite.Suite
	db          mongodb.SourceInterface
	repository  OrderRepositoryInterface
	logObserver *zap.Logger
	zapRecorder *observer.ObservedLogs
}

func Test_Order(t *testing.T) {
	suite.Run(t, new(OrderTestSuite))
}

func (suite *OrderTestSuite) SetupTest() {
	_, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)
	zap.ReplaceGlobals(suite.logObserver)

	suite.db, err = mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	suite.repository = NewOrderRepository(suite.db)
}

func (suite *OrderTestSuite) TearDownTest() {
	if err := suite.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	if err := suite.db.Close(); err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *CountryTestSuite) TestCountry_NewOrderRepository_Ok() {
	repository := NewOrderRepository(suite.db)
	assert.IsType(suite.T(), &orderRepository{}, repository)
}

func (suite *OrderTestSuite) TestOrder_Insert_Ok() {
	order := &billingpb.Order{
		Id: primitive.NewObjectID().Hex(),
		Project: &billingpb.ProjectOrder{
			Id:         primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
		},
	}
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetById(context.TODO(), order.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), order.Id, order2.Id)
}

// TODO: Use the DB mock for return error on insert entry
func (suite *OrderTestSuite) TestOrder_Insert_Error() {
	order := &billingpb.Order{
		Id: primitive.NewObjectID().Hex(),
		Project: &billingpb.ProjectOrder{
			Id:         primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
		},
		CreatedAt: &timestamp.Timestamp{Seconds: -100000000000000},
	}
	err := suite.repository.Insert(context.TODO(), order)
	assert.Error(suite.T(), err)
}

// TODO: Use the DB mock for to skip really inserting the entry to DB
func (suite *OrderTestSuite) TestOrder_Insert_DontHaveDbErrorButDontInserted() {
	refund, err := suite.repository.GetById(context.TODO(), primitive.NewObjectID().Hex())
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), refund)
}

func (suite *OrderTestSuite) TestOrder_Update_Ok() {
	order := &billingpb.Order{
		Id: primitive.NewObjectID().Hex(),
		Project: &billingpb.ProjectOrder{
			Id:         primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
		},
		MccCode: "code1",
	}
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	refund1, err := suite.repository.GetById(context.TODO(), order.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), order.Id, refund1.Id)
	assert.Equal(suite.T(), order.MccCode, refund1.MccCode)

	order.MccCode = "code2"
	err = suite.repository.Update(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetById(context.TODO(), order.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), order.Id, order2.Id)
	assert.Equal(suite.T(), order.MccCode, order2.MccCode)
}

// TODO: Use the DB mock for return error on insert entry
func (suite *OrderTestSuite) TestOrder_Update_Error() {
	order := &billingpb.Order{
		Id: primitive.NewObjectID().Hex(),
		Project: &billingpb.ProjectOrder{
			Id:         primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
		},
		MccCode: "code1",
	}
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order.CreatedAt = &timestamp.Timestamp{Seconds: -100000000000000}
	err = suite.repository.Update(context.TODO(), order)
	assert.Error(suite.T(), err)
}

// TODO: Use the DB mock for to skip really updating the entry to DB
func (suite *OrderTestSuite) TestOrder_Update_DontHaveDbErrorButDontUpdated() {
	order := &billingpb.Order{
		Id: primitive.NewObjectID().Hex(),
		Project: &billingpb.ProjectOrder{
			Id:         primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
		},
		MccCode: "code1",
	}
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order1, err := suite.repository.GetById(context.TODO(), order.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), order.Id, order1.Id)
	assert.Equal(suite.T(), order.MccCode, order1.MccCode)

	order.MccCode = "test2"
	// TODO: Use the mock of DB
	//err = suite.repository.Update(context.TODO(), order)
	//assert.NoError(suite.T(), err)

	refund2, err := suite.repository.GetById(context.TODO(), order.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), order.Id, refund2.Id)
	assert.NotEqual(suite.T(), order.MccCode, refund2.MccCode)
}

func (suite *OrderTestSuite) TestOrder_GetById_Ok() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetById(context.TODO(), order.Id)
	assert.NoError(suite.T(), err)

	order.CreatedAt = order2.CreatedAt
	order.UpdatedAt = order2.UpdatedAt
	order.CanceledAt = order2.CanceledAt
	order.RefundedAt = order2.RefundedAt
	order.ProjectLastRequestedAt = order2.ProjectLastRequestedAt
	order.PaymentMethodOrderClosedAt = order2.PaymentMethodOrderClosedAt
	order.ExpireDateToFormInput = order2.ExpireDateToFormInput
	order.Project.FirstPaymentAt = order2.Project.FirstPaymentAt

	assert.Equal(suite.T(), order, order2)
}

func (suite *OrderTestSuite) TestOrder_GetById_ErrorNotFound() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetById(context.TODO(), order.Uuid)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order2)
}

func (suite *OrderTestSuite) TestOrder_GetByUuid_Ok() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetByUuid(context.TODO(), order.Uuid)
	assert.NoError(suite.T(), err)

	order.CreatedAt = order2.CreatedAt
	order.UpdatedAt = order2.UpdatedAt
	order.CanceledAt = order2.CanceledAt
	order.RefundedAt = order2.RefundedAt
	order.ProjectLastRequestedAt = order2.ProjectLastRequestedAt
	order.PaymentMethodOrderClosedAt = order2.PaymentMethodOrderClosedAt
	order.ExpireDateToFormInput = order2.ExpireDateToFormInput
	order.Project.FirstPaymentAt = order2.Project.FirstPaymentAt
	assert.Equal(suite.T(), order, order2)
}

func (suite *OrderTestSuite) TestOrder_GetByUuid_Error() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetByUuid(context.TODO(), order.Id)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order2)
}

func (suite *OrderTestSuite) TestOrder_GetByRefundReceiptNumber_Ok() {
	order := suite.getOrderTemplate()
	order.Refund = &billingpb.OrderNotificationRefund{ReceiptNumber: "number"}
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetByRefundReceiptNumber(context.TODO(), order.Refund.ReceiptNumber)
	assert.NoError(suite.T(), err)

	order.CreatedAt = order2.CreatedAt
	order.UpdatedAt = order2.UpdatedAt
	order.CanceledAt = order2.CanceledAt
	order.RefundedAt = order2.RefundedAt
	order.ProjectLastRequestedAt = order2.ProjectLastRequestedAt
	order.PaymentMethodOrderClosedAt = order2.PaymentMethodOrderClosedAt
	order.ExpireDateToFormInput = order2.ExpireDateToFormInput
	order.Project.FirstPaymentAt = order2.Project.FirstPaymentAt
	assert.Equal(suite.T(), order, order2)
}

func (suite *OrderTestSuite) TestOrder_GetByRefundReceiptNumber_Error() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order2, err := suite.repository.GetByRefundReceiptNumber(context.TODO(), order.Uuid)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order2)
}

func (suite *OrderTestSuite) TestOrder_GetByUUidAndMerchant_Ok() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order1, err := suite.repository.GetByUuidAndMerchantId(context.TODO(), order.Uuid, order.GetMerchantId())
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), order.Id, order1.Id)
	assert.Equal(suite.T(), order.Uuid, order1.Uuid)
	assert.Equal(suite.T(), order.Project, order1.Project)
}

func (suite *OrderTestSuite) TestOrder_GetByUUidAndMerchant_ErrorNotFound() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order1, err := suite.repository.GetByUuidAndMerchantId(context.TODO(), "ace2fc5c-b8c2-4424-96e8-5b631a73b88a", order.GetMerchantId())
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order1)

	messages := suite.zapRecorder.All()
	assert.Len(suite.T(), messages, 1)
	assert.Equal(suite.T(), zap.ErrorLevel, messages[0].Level)
	assert.Equal(suite.T(), pkg.ErrorDatabaseQueryFailed, messages[0].Message)
}

func (suite *OrderTestSuite) TestOrder_GetByUUidAndMerchant_IncorrectMerchantId_Error() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	order1, err := suite.repository.GetByUuidAndMerchantId(context.TODO(), order.Uuid, "ace2fc5c-b8c2-4424-96e8-5b631a73b88a")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order1)

	messages := suite.zapRecorder.All()
	assert.Len(suite.T(), messages, 1)
	assert.Equal(suite.T(), zap.ErrorLevel, messages[0].Level)
	assert.Equal(suite.T(), pkg.ErrorDatabaseInvalidObjectId, messages[0].Message)
}

func (suite *OrderTestSuite) TestOrder_GetByUUidAndMerchant_Mapper_Error() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	mapperMock := &mocks.Mapper{}
	mapperMock.On("MapMgoToObject", mock.Anything).Return(nil, errors.New("some error"))
	suite.repository.(*orderRepository).mapper = mapperMock

	order1, err := suite.repository.GetByUuidAndMerchantId(context.TODO(), order.Uuid, order.GetMerchantId())
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order1)

	messages := suite.zapRecorder.All()
	assert.Len(suite.T(), messages, 1)
	assert.Equal(suite.T(), zap.ErrorLevel, messages[0].Level)
	assert.Equal(suite.T(), pkg.ErrorMapModelFailed, messages[0].Message)
}

func (suite *OrderTestSuite) TestOrder_GetBySubscriptionId() {
	order := suite.getOrderTemplate()
	err := suite.repository.Insert(context.TODO(), order)
	assert.NoError(suite.T(), err)

	orders, err := suite.repository.GetBySubscriptionId(context.TODO(), order.RecurringSubscriptionId)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), orders, 1)
	assert.Equal(suite.T(), order.Id, orders[0].Id)
	assert.Equal(suite.T(), order.RecurringSubscriptionId, orders[0].RecurringSubscriptionId)
}

func (suite *OrderTestSuite) getOrderTemplate() *billingpb.Order {

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
