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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type PaymentChannelCostSystemTestSuite struct {
	suite.Suite
	service                    *Service
	log                        *zap.Logger
	cache                      database.CacheInterface
	paymentChannelCostSystemId string
	operatingCompany           *billingpb.OperatingCompany
}

func Test_PaymentChannelCostSystem(t *testing.T) {
	suite.Run(t, new(PaymentChannelCostSystemTestSuite))
}

func (suite *PaymentChannelCostSystemTestSuite) SetupTest() {
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
		&casbinMocks.CasbinService{},
		mocks.NewNotifierOk(),
		mocks.NewBrokerMockOk(),
		mocks.NewBrokerMockOk(),
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.operatingCompany = HelperOperatingCompany(suite.Suite, suite.service)

	countryAz := &billingpb.Country{
		Id:                primitive.NewObjectID().Hex(),
		IsoCodeA2:         "AZ",
		Region:            "CIS",
		Currency:          "AZN",
		PaymentsAllowed:   true,
		ChangeAllowed:     true,
		VatEnabled:        true,
		PriceGroupId:      "",
		VatCurrency:       "AZN",
		PayerTariffRegion: billingpb.TariffRegionRussiaAndCis,
	}
	countryUs := &billingpb.Country{
		Id:                primitive.NewObjectID().Hex(),
		IsoCodeA2:         "US",
		Region:            "US",
		Currency:          "USD",
		PaymentsAllowed:   true,
		ChangeAllowed:     true,
		VatEnabled:        true,
		PriceGroupId:      "",
		VatCurrency:       "USD",
		PayerTariffRegion: billingpb.TariffRegionWorldwide,
	}
	countries := []*billingpb.Country{countryAz, countryUs}
	if err := suite.service.country.MultipleInsert(context.TODO(), countries); err != nil {
		suite.FailNow("Insert country test data failed", "%v", err)
	}

	suite.paymentChannelCostSystemId = primitive.NewObjectID().Hex()

	paymentChannelCostSystem := &billingpb.PaymentChannelCostSystem{
		Id:                 suite.paymentChannelCostSystemId,
		Name:               "VISA",
		Region:             billingpb.TariffRegionRussiaAndCis,
		Country:            "AZ",
		Percent:            1.5,
		FixAmount:          5,
		FixAmountCurrency:  "USD",
		IsActive:           false,
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}

	anotherPaymentChannelCostSystem := &billingpb.PaymentChannelCostSystem{
		Name:               "VISA",
		Region:             billingpb.TariffRegionRussiaAndCis,
		Country:            "",
		Percent:            2.2,
		FixAmount:          0,
		FixAmountCurrency:  "USD",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}
	pccs := []*billingpb.PaymentChannelCostSystem{paymentChannelCostSystem, anotherPaymentChannelCostSystem}
	if err := suite.service.paymentChannelCostSystemRepository.MultipleInsert(context.TODO(), pccs); err != nil {
		suite.FailNow("Insert PaymentChannelCostSystem test data failed", "%v", err)
	}
}

func (suite *PaymentChannelCostSystemTestSuite) TearDownTest() {
	suite.cache.FlushAll()
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *PaymentChannelCostSystemTestSuite) TestPaymentChannelCostSystem_GrpcGet_Ok() {
	req := &billingpb.PaymentChannelCostSystemRequest{
		Name:               "VISA",
		Region:             billingpb.TariffRegionRussiaAndCis,
		Country:            "AZ",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}

	res := &billingpb.PaymentChannelCostSystemResponse{}

	err := suite.service.GetPaymentChannelCostSystem(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), res.Item.Country, "AZ")
	assert.Equal(suite.T(), res.Item.FixAmount, float64(5))

	req.Country = ""
	err = suite.service.GetPaymentChannelCostSystem(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), res.Item.Country, "")
	assert.Equal(suite.T(), res.Item.FixAmount, float64(0))
}

func (suite *PaymentChannelCostSystemTestSuite) TestPaymentChannelCostSystem_GrpcSet_Ok() {
	req := &billingpb.PaymentChannelCostSystem{
		Name:               "VISA",
		Region:             billingpb.TariffRegionRussiaAndCis,
		Country:            "AZ",
		Percent:            1.7,
		FixAmount:          4,
		FixAmountCurrency:  "USD",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}

	res := billingpb.PaymentChannelCostSystemResponse{}

	err := suite.service.SetPaymentChannelCostSystem(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), res.Item.Country, "AZ")
	assert.Equal(suite.T(), res.Item.FixAmount, float64(4))
	assert.Equal(suite.T(), res.Item.Id, suite.paymentChannelCostSystemId)

	req2 := &billingpb.PaymentChannelCostSystem{
		Name:               "MASTERCARD",
		Region:             billingpb.TariffRegionWorldwide,
		Country:            "",
		Percent:            2.2,
		FixAmount:          1,
		FixAmountCurrency:  "USD",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}

	res2 := billingpb.PaymentChannelCostSystemResponse{}
	err = suite.service.SetPaymentChannelCostSystem(context.TODO(), req2, &res2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), "", res2.Item.Country)
	assert.Equal(suite.T(), billingpb.TariffRegionWorldwide, res2.Item.Region)
	assert.Equal(suite.T(), float64(1), res2.Item.FixAmount)
	assert.NotEqual(suite.T(), suite.paymentChannelCostSystemId, res2.Item.Id)
}

func (suite *PaymentChannelCostSystemTestSuite) TestPaymentChannelCostSystem_Delete_Ok() {
	req := &billingpb.PaymentCostDeleteRequest{
		Id: suite.paymentChannelCostSystemId,
	}

	res := &billingpb.ResponseError{}
	err := suite.service.DeletePaymentChannelCostSystem(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)

	_, err = suite.service.paymentChannelCostSystemRepository.GetById(context.TODO(), suite.paymentChannelCostSystemId)
	assert.Error(suite.T(), err)
}

func (suite *PaymentChannelCostSystemTestSuite) TestPaymentChannelCostSystem_GetAllPaymentChannelCostSystem_Ok() {
	res := &billingpb.PaymentChannelCostSystemListResponse{}
	err := suite.service.GetAllPaymentChannelCostSystem(context.TODO(), &billingpb.EmptyRequest{}, res)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), len(res.Item.Items), 2)
}
