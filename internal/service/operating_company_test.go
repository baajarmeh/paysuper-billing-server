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

type OperatingCompanyTestSuite struct {
	suite.Suite
	service           *Service
	log               *zap.Logger
	cache             database.CacheInterface
	operatingCompany  *billingpb.OperatingCompany
	operatingCompany2 *billingpb.OperatingCompany
}

func Test_OperatingCompany(t *testing.T) {
	suite.Run(t, new(OperatingCompanyTestSuite))
}

func (suite *OperatingCompanyTestSuite) SetupTest() {
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
		mocks.NewBrokerMockOk(),
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	countryRu := &billingpb.Country{
		Id:                primitive.NewObjectID().Hex(),
		IsoCodeA2:         "RU",
		Region:            "Russia",
		Currency:          "RUB",
		PaymentsAllowed:   true,
		ChangeAllowed:     true,
		VatEnabled:        true,
		PriceGroupId:      primitive.NewObjectID().Hex(),
		VatCurrency:       "RUB",
		PayerTariffRegion: billingpb.TariffRegionRussiaAndCis,
	}
	countryUa := &billingpb.Country{
		Id:                primitive.NewObjectID().Hex(),
		IsoCodeA2:         "UA",
		Region:            "UA",
		Currency:          "UAH",
		PaymentsAllowed:   true,
		ChangeAllowed:     true,
		VatEnabled:        false,
		PriceGroupId:      "",
		VatCurrency:       "",
		PayerTariffRegion: billingpb.TariffRegionRussiaAndCis,
	}
	countries := []*billingpb.Country{countryRu, countryUa}
	if err := suite.service.country.MultipleInsert(context.TODO(), countries); err != nil {
		suite.FailNow("Insert country test data failed", "%v", err)
	}

	suite.operatingCompany = &billingpb.OperatingCompany{
		Name:               "Legal name",
		Country:            "RU",
		RegistrationNumber: "some number",
		VatNumber:          "some vat number",
		Address:            "Home, home 0",
		VatAddress:         "Address for VAT purposes",
		SignatoryName:      "Vassiliy Poupkine",
		SignatoryPosition:  "CEO",
		BankingDetails:     "bank details including bank, bank address, account number, swift/ bic, intermediary bank",
		PaymentCountries:   []string{},
	}

	suite.operatingCompany2 = &billingpb.OperatingCompany{
		Name:               "Legal name 2",
		Country:            "ML",
		RegistrationNumber: "some number 2",
		VatNumber:          "some vat number 2",
		Address:            "Home, home 1",
		VatAddress:         "Address for VAT purposes 2",
		SignatoryName:      "Ivan Petroff",
		SignatoryPosition:  "CEO",
		BankingDetails:     "bank details including bank, bank address, account number, swift/ bic, intermediary bank",
		PaymentCountries:   []string{"RU", "UA"},
	}
}

func (suite *OperatingCompanyTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *OperatingCompanyTestSuite) Test_OperatingCompany_AddOk() {
	list, err := suite.service.operatingCompanyRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), list)

	res := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.AddOperatingCompany(context.TODO(), suite.operatingCompany, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)

	list, err = suite.service.operatingCompanyRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), list)
	assert.Len(suite.T(), list, 1)
}

func (suite *OperatingCompanyTestSuite) Test_OperatingCompany_ListOk() {
	res := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.AddOperatingCompany(context.TODO(), suite.operatingCompany, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)

	res2 := &billingpb.GetOperatingCompaniesListResponse{}
	err = suite.service.GetOperatingCompaniesList(context.TODO(), &billingpb.EmptyRequest{}, res2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)
	assert.Len(suite.T(), res2.Items, 1)
}

func (suite *OperatingCompanyTestSuite) Test_OperatingCompany_AddFail_DuplicatePaymentCountry() {
	list, err := suite.service.operatingCompanyRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), list)

	res := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.AddOperatingCompany(context.TODO(), suite.operatingCompany, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)

	list, err = suite.service.operatingCompanyRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), list)
	assert.Len(suite.T(), list, 1)

	err = suite.service.AddOperatingCompany(context.TODO(), suite.operatingCompany, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusBadData)
	assert.Equal(suite.T(), res.Message, errorOperatingCompanyCountryAlreadyExists)
}

func (suite *OperatingCompanyTestSuite) Test_OperatingCompany_AddFail_PaymentCountryUnknown() {
	list, err := suite.service.operatingCompanyRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), list)

	suite.operatingCompany.PaymentCountries = []string{"RU", "UA", "XXX"}

	res := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.AddOperatingCompany(context.TODO(), suite.operatingCompany, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusBadData)
	assert.Equal(suite.T(), res.Message, errorOperatingCompanyCountryUnknown)
}
