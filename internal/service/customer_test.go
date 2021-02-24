package service

import (
	"context"
	"errors"
	"github.com/go-redis/redis"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	mocks2 "github.com/paysuper/paysuper-proto/go/recurringpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
	"time"
)

type CustomerTestSuite struct {
	suite.Suite
	service  *Service
	cache    database.CacheInterface
	customer *billingpb.Customer
}

func Test_Customer(t *testing.T) {
	suite.Run(t, new(CustomerTestSuite))
}

func (suite *CustomerTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	db, err := mongodb.NewDatabase()
	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	m, err := migrate.New("file://../../migrations/tests", cfg.MongoDsn)
	if err != nil {
		suite.FailNow("Migrate init failed", "%v", err)
	}

	err = m.Up()
	if err != nil && err.Error() != "no change" {
		suite.FailNow("Migrations failed", "%v", err)
	}

	redisClient := database.NewRedis(
		&redis.Options{
			Addr:     cfg.RedisHost,
			Password: cfg.RedisPassword,
		},
	)

	redisdb := mocks.NewTestRedis()
	suite.cache, err = database.NewCacheRedis(redisdb, "cache")

	if err != nil {
		suite.FailNow("Cache redis initialize failed", "%v", err)
	}

	suite.service = NewBillingService(
		db,
		cfg,
		mocks.NewGeoIpServiceTestOk(),
		nil,
		nil,
		nil,
		redisClient,
		suite.cache,
		mocks.NewCurrencyServiceMockOk(),
		mocks.NewDocumentSignerMockOk(),
		&reportingMocks.ReporterService{},
		mocks.NewFormatterOK(),
		mocks.NewBrokerMockOk(),
		&casbinMocks.CasbinService{},
		nil,
		mocks.NewBrokerMockOk(),
		mocks.NewBrokerMockOk(),
	)

	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}
}

func (suite *CustomerTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *CustomerTestSuite) TestCustomer_SetCustomerPaymentActivity_OrderTypePayment_Ok() {
	t := time.Now().Add(-10 * time.Hour)
	pt, err := ptypes.TimestampProto(t)
	assert.NoError(suite.T(), err)

	req := &billingpb.SetCustomerPaymentActivityRequest{
		CustomerId:   "ffffffffffffffffffffffff",
		MerchantId:   "fffffffffffffffffffffff0",
		Type:         billingpb.OrderTypeOrder,
		ProcessingAt: pt,
		Amount:       10.23,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}

	for i := 0; i < 5; i++ {
		err = suite.service.SetCustomerPaymentActivity(context.TODO(), req, rsp)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	}

	customer, err := suite.service.customerRepository.GetById(ctx, req.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)
	assert.NotNil(suite.T(), customer.PaymentActivity)
	assert.Len(suite.T(), customer.PaymentActivity, 1)
	assert.Contains(suite.T(), customer.PaymentActivity, req.MerchantId)
	assert.EqualValues(suite.T(), 5, customer.PaymentActivity[req.MerchantId].Count.Payment)
	assert.EqualValues(suite.T(), 0, customer.PaymentActivity[req.MerchantId].Count.Refund)
	assert.EqualValues(suite.T(), 10.23, customer.PaymentActivity[req.MerchantId].Revenue.Payment)
	assert.EqualValues(suite.T(), 0, customer.PaymentActivity[req.MerchantId].Revenue.Refund)
	assert.EqualValues(suite.T(), 10.23*5, customer.PaymentActivity[req.MerchantId].Revenue.Total)
	assert.Equal(suite.T(), req.ProcessingAt.Seconds, customer.PaymentActivity[req.MerchantId].LastTxnAt.Payment.Seconds)
	assert.Zero(suite.T(), customer.PaymentActivity[req.MerchantId].LastTxnAt.Refund.Seconds)

	req.Amount = 7.67
	req.Type = billingpb.OrderTypeRefund

	for i := 0; i < 3; i++ {
		err = suite.service.SetCustomerPaymentActivity(context.TODO(), req, rsp)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	}

	customer, err = suite.service.customerRepository.GetById(ctx, req.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)
	assert.NotNil(suite.T(), customer.PaymentActivity)
	assert.Len(suite.T(), customer.PaymentActivity, 1)
	assert.Contains(suite.T(), customer.PaymentActivity, req.MerchantId)
	assert.EqualValues(suite.T(), 5, customer.PaymentActivity[req.MerchantId].Count.Payment)
	assert.EqualValues(suite.T(), 3, customer.PaymentActivity[req.MerchantId].Count.Refund)
	assert.EqualValues(suite.T(), 10.23, customer.PaymentActivity[req.MerchantId].Revenue.Payment)
	assert.EqualValues(suite.T(), 7.67, customer.PaymentActivity[req.MerchantId].Revenue.Refund)
	assert.EqualValues(suite.T(), 10.23*5-7.67*3, customer.PaymentActivity[req.MerchantId].Revenue.Total)
	assert.Equal(suite.T(), req.ProcessingAt.Seconds, customer.PaymentActivity[req.MerchantId].LastTxnAt.Payment.Seconds)
	assert.Equal(suite.T(), req.ProcessingAt.Seconds, customer.PaymentActivity[req.MerchantId].LastTxnAt.Refund.Seconds)

	req.Type = billingpb.OrderTypeOrder
	req.MerchantId = "fffffffffffffffffffffff2"
	err = suite.service.SetCustomerPaymentActivity(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)

	customer, err = suite.service.customerRepository.GetById(ctx, req.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)
	assert.NotNil(suite.T(), customer.PaymentActivity)
	assert.Len(suite.T(), customer.PaymentActivity, 2)
	assert.Contains(suite.T(), customer.PaymentActivity, "fffffffffffffffffffffff0")
	assert.Contains(suite.T(), customer.PaymentActivity, req.MerchantId)
	assert.EqualValues(suite.T(), 1, customer.PaymentActivity[req.MerchantId].Count.Payment)
	assert.EqualValues(suite.T(), 0, customer.PaymentActivity[req.MerchantId].Count.Refund)
	assert.EqualValues(suite.T(), 7.67, customer.PaymentActivity[req.MerchantId].Revenue.Payment)
	assert.EqualValues(suite.T(), 0, customer.PaymentActivity[req.MerchantId].Revenue.Refund)
	assert.EqualValues(suite.T(), 7.67, customer.PaymentActivity[req.MerchantId].Revenue.Total)
	assert.Equal(suite.T(), req.ProcessingAt.Seconds, customer.PaymentActivity[req.MerchantId].LastTxnAt.Payment.Seconds)
	assert.Zero(suite.T(), customer.PaymentActivity[req.MerchantId].LastTxnAt.Refund.Seconds)
}

func (suite *CustomerTestSuite) TestCustomer_SetCustomerPaymentActivity_CustomerOrderTypeNotSupport_Error() {
	req := &billingpb.SetCustomerPaymentActivityRequest{
		CustomerId:   "ffffffffffffffffffffffff",
		MerchantId:   "fffffffffffffffffffffff0",
		Type:         "unknown_type",
		ProcessingAt: ptypes.TimestampNow(),
		Amount:       10.23,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.SetCustomerPaymentActivity(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), errorCustomerOrderTypeNotSupport, rsp.Message)
}

func (suite *CustomerTestSuite) TestCustomer_SetCustomerPaymentActivity_CustomerRepository_GetById_Error() {
	customerRepositoryMock := &mocks.CustomerRepositoryInterface{}
	customerRepositoryMock.On("GetById", mock.Anything, mock.Anything).
		Return(nil, errors.New("TestCustomer_SetCustomerPaymentActivity_CustomerRepository_GetById_Error"))
	suite.service.customerRepository = customerRepositoryMock

	req := &billingpb.SetCustomerPaymentActivityRequest{
		CustomerId:   "ffffffffffffffffffffffff",
		MerchantId:   "fffffffffffffffffffffff0",
		Type:         billingpb.OrderTypeOrder,
		ProcessingAt: ptypes.TimestampNow(),
		Amount:       10.23,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.SetCustomerPaymentActivity(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), customerNotFound, rsp.Message)
}

func (suite *CustomerTestSuite) TestCustomer_SetCustomerPaymentActivity_CustomerRepository_Update_Error() {
	customer, err := suite.service.customerRepository.GetById(ctx, "ffffffffffffffffffffffff")
	assert.NoError(suite.T(), err)

	customerRepositoryMock := &mocks.CustomerRepositoryInterface{}
	customerRepositoryMock.On("GetById", mock.Anything, mock.Anything).Return(customer, nil)
	customerRepositoryMock.On("Update", mock.Anything, mock.Anything).
		Return(errors.New("TestCustomer_SetCustomerPaymentActivity_CustomerRepository_Update_Error"))
	suite.service.customerRepository = customerRepositoryMock

	req := &billingpb.SetCustomerPaymentActivityRequest{
		CustomerId:   "ffffffffffffffffffffffff",
		MerchantId:   "fffffffffffffffffffffff0",
		Type:         billingpb.OrderTypeOrder,
		ProcessingAt: ptypes.TimestampNow(),
		Amount:       10.23,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.SetCustomerPaymentActivity(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), errorCustomerUnknown, rsp.Message)
}

func (suite *CustomerTestSuite) TestCustomer_DeleteCard_Error() {
	repMock := &mocks2.RepositoryService{}
	repMock.On("DeleteSavedCard", mock.Anything, mock.Anything).Return(nil, errors.New("some error"))
	suite.service.rep = repMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteCustomerCard(context.TODO(), &billingpb.DeleteCustomerCardRequest{}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, rsp.Message)
}

func (suite *CustomerTestSuite) TestCustomer_DeleteCard_ServiceError() {
	repMock := &mocks2.RepositoryService{}
	repMock.On("DeleteSavedCard", mock.Anything, mock.Anything).Return(&recurringpb.DeleteSavedCardResponse{Status: 404, Message: "hello there"}, nil)
	suite.service.rep = repMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteCustomerCard(context.TODO(), &billingpb.DeleteCustomerCardRequest{}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), recurringSavedCardNotFount, rsp.Message)
}

func (suite *CustomerTestSuite) TestCustomer_DeleteCard_Ok() {
	repMock := &mocks2.RepositoryService{}
	repMock.On("DeleteSavedCard", mock.Anything, mock.Anything).Return(&recurringpb.DeleteSavedCardResponse{Status: 200}, nil)
	suite.service.rep = repMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteCustomerCard(context.TODO(), &billingpb.DeleteCustomerCardRequest{}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
}
