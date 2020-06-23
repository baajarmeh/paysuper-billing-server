package service

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type PaymentMinLimitSystemTestSuite struct {
	suite.Suite
	service                *Service
	log                    *zap.Logger
	cache                  database.CacheInterface
	PaymentMinLimitSystem  *billingpb.PaymentMinLimitSystem
	PaymentMinLimitSystem2 *billingpb.PaymentMinLimitSystem
}

func Test_PaymentMinLimitSystem(t *testing.T) {
	suite.Run(t, new(PaymentMinLimitSystemTestSuite))
}

func (suite *PaymentMinLimitSystemTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	db, err := mongodb.NewDatabase()
	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	suite.log, err = zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	redisdb := mocks.NewTestRedis()
	suite.cache, err = database.NewCacheRedis(redisdb, "cache")

	if err != nil {
		suite.FailNow("Cache redis initialize failed", "%v", err)
	}

	casbin := &casbinMocks.CasbinService{}

	suite.service = NewBillingService(
		db,
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.cache,
		mocks.NewCurrencyServiceMockOk(),
		mocks.NewDocumentSignerMockOk(),
		&reportingMocks.ReporterService{},
		mocks.NewFormatterOK(),
		mocks.NewBrokerMockOk(),
		casbin,
		nil,
		mocks.NewBrokerMockOk(),
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.PaymentMinLimitSystem = &billingpb.PaymentMinLimitSystem{
		Currency: "RUB",
		Amount:   150,
	}

	suite.PaymentMinLimitSystem2 = &billingpb.PaymentMinLimitSystem{
		Currency: "USD",
		Amount:   2,
	}
}

func (suite *PaymentMinLimitSystemTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *PaymentMinLimitSystemTestSuite) Test_PaymentMinLimitSystem_AddOk() {
	list, err := suite.service.paymentMinLimitSystemRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 0)

	res := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.SetPaymentMinLimitSystem(context.TODO(), suite.PaymentMinLimitSystem, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)

	list, err = suite.service.paymentMinLimitSystemRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
}

func (suite *PaymentMinLimitSystemTestSuite) Test_PaymentMinLimitSystem_ListOk() {
	res := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.SetPaymentMinLimitSystem(context.TODO(), suite.PaymentMinLimitSystem, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)

	res2 := &billingpb.GetPaymentMinLimitsSystemResponse{}
	err = suite.service.GetPaymentMinLimitsSystem(context.TODO(), &billingpb.EmptyRequest{}, res2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)
	assert.Len(suite.T(), res2.Items, 1)
}

func (suite *PaymentMinLimitSystemTestSuite) Test_PaymentMinLimitSystem_AddFail_PaymentCountryUnknown() {
	list, err := suite.service.paymentMinLimitSystemRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 0)

	suite.PaymentMinLimitSystem.Currency = "XXX"

	res := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.SetPaymentMinLimitSystem(context.TODO(), suite.PaymentMinLimitSystem, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusBadData)
	assert.Equal(suite.T(), res.Message, errorPaymentMinLimitSystemCurrencyUnknown)
}
