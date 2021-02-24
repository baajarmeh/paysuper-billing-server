package service

import (
	"context"
	"errors"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type PaymentMethodTestSuite struct {
	suite.Suite
	service          *Service
	log              *zap.Logger
	cache            database.CacheInterface
	pmQiwi           *billingpb.PaymentMethod
	project          *billingpb.Project
	operatingCompany *billingpb.OperatingCompany
}

func Test_PaymentMethod(t *testing.T) {
	suite.Run(t, new(PaymentMethodTestSuite))
}

func (suite *PaymentMethodTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	db, err := mongodb.NewDatabase()
	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	suite.operatingCompany = &billingpb.OperatingCompany{
		Id:                 primitive.NewObjectID().Hex(),
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

	ps := &billingpb.PaymentSystem{
		Id:                 primitive.NewObjectID().Hex(),
		Name:               "CardPay",
		AccountingCurrency: "RUB",
		AccountingPeriod:   "every-day",
		Country:            "",
		IsActive:           true,
		Handler:            "cardpay",
	}

	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")

	suite.pmQiwi = &billingpb.PaymentMethod{
		LimitsCurrency:   "EUR",
		Id:               primitive.NewObjectID().Hex(),
		Name:             "Qiwi",
		Group:            "QIWI",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		ExternalId:       "QIWI",
		TestSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "RUB",
				TerminalId:         "15993",
				Secret:             "A1tph4I6BD0f",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
			},
		},
		Type:            "ewallet",
		IsActive:        true,
		PaymentSystemId: ps.Id,
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
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	pms := []*billingpb.PaymentMethod{suite.pmQiwi}
	if err := suite.service.paymentMethodRepository.MultipleInsert(context.TODO(), pms); err != nil {
		suite.FailNow("Insert payment methods test data failed", "%v", err)
	}

	if err := suite.service.paymentSystemRepository.Insert(context.TODO(), ps); err != nil {
		suite.FailNow("Insert payment system test data failed", "%v", err)
	}

	project := &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         "default",
		LimitsCurrency:           "USD",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       false,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   billingpb.ProjectStatusDraft,
		MerchantId:               primitive.NewObjectID().Hex(),
	}
	suite.project = project
	err = suite.service.project.Insert(context.TODO(), project)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
	}

	err = suite.service.operatingCompanyRepository.Upsert(context.TODO(), suite.operatingCompany)

	if err != nil {
		suite.FailNow("Insert operatingCompany test data failed", "%v", err)
	}
}

func (suite *PaymentMethodTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentSettings_ErrorNoTestSettings() {
	method := &billingpb.PaymentMethod{LimitsCurrency: "EUR"}
	_, err := suite.service.getPaymentSettings(method, "RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "VISA", false)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), err, orderErrorPaymentMethodEmptySettings)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentSettings_OkTestSettings() {
	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "VISA")

	method := &billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		Id:             primitive.NewObjectID().Hex(),
		Name:           "Unit Test",
		Group:          "Unit",
		ExternalId:     "Unit",
		ProductionSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "RUB",
				TerminalId:         "15993",
				Secret:             "A1tph4I6BD0f",
				SecretCallback:     "A1tph4I6BD0f",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
			},
		},
		Type:            "ewallet",
		IsActive:        true,
		PaymentSystemId: suite.pmQiwi.PaymentSystemId,
	}
	err := suite.service.paymentMethodRepository.Insert(context.TODO(), method)
	assert.NoError(suite.T(), err)

	suite.project.Status = billingpb.ProjectStatusInProduction
	err = suite.service.project.Update(context.TODO(), suite.project)
	assert.NoError(suite.T(), err)

	settings, err := suite.service.getPaymentSettings(method, "RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "VISA", suite.project.IsProduction())
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), method.ProductionSettings[key].Secret, settings.Secret)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentSettings_ErrorNoPaymentCurrency() {
	method := &billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		ProductionSettings: map[string]*billingpb.PaymentMethodParams{
			"RUB": {Currency: "RUB"},
		},
	}
	project := &billingpb.Project{
		Status: billingpb.ProjectStatusInProduction,
	}
	_, err := suite.service.getPaymentSettings(method, "EUR", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "VISA", project.IsProduction())
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), err, orderErrorPaymentMethodEmptySettings)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentSettings_Ok() {
	key := helper.GetPaymentMethodKey("EUR", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "VISA")

	method := &billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		Id:             primitive.NewObjectID().Hex(),
		Name:           "Unit Test",
		Group:          "Unit",
		ExternalId:     "Unit",
		TestSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "EUR",
				TerminalId:         "15993",
				Secret:             "A1tph4I6BD0f",
				SecretCallback:     "A1tph4I6BD0f",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
			},
		},
		Type:            "ewallet",
		IsActive:        true,
		PaymentSystemId: suite.pmQiwi.PaymentSystemId,
	}
	err := suite.service.paymentMethodRepository.Insert(context.TODO(), method)
	assert.NoError(suite.T(), err)

	settings, err := suite.service.getPaymentSettings(method, "EUR", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "VISA", suite.project.IsProduction())
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), method.TestSettings[key].Secret, settings.Secret)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethod_ErrorPaymentSystem() {
	req := &billingpb.PaymentMethod{
		LimitsCurrency:  "EUR",
		PaymentSystemId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.ChangePaymentMethodResponse{}
	paymentSystem := &mocks.PaymentSystemRepositoryInterface{}

	paymentSystem.On("GetById", mock2.Anything, req.PaymentSystemId).Return(nil, errors.New("not found"))
	suite.service.paymentSystemRepository = paymentSystem

	err := suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethod_ErrorInvalidId() {
	req := &billingpb.PaymentMethod{
		LimitsCurrency:  "EUR",
		Id:              primitive.NewObjectID().Hex(),
		PaymentSystemId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.ChangePaymentMethodResponse{}
	paymentSystem := &mocks.PaymentSystemRepositoryInterface{}

	paymentSystem.On("GetById", mock2.Anything, req.PaymentSystemId).Return(&billingpb.PaymentSystem{}, nil)
	suite.service.paymentSystemRepository = paymentSystem

	err := suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethod_ErrorActivate() {
	req := &billingpb.PaymentMethod{
		LimitsCurrency:  "EUR",
		PaymentSystemId: primitive.NewObjectID().Hex(),
		IsActive:        true,
	}
	rsp := &billingpb.ChangePaymentMethodResponse{}
	paymentSystem := &mocks.PaymentSystemRepositoryInterface{}

	paymentSystem.On("GetById", mock2.Anything, req.PaymentSystemId).Return(&billingpb.PaymentSystem{}, nil)
	suite.service.paymentSystemRepository = paymentSystem

	err := suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)

	req.ExternalId = "externalId"
	err = suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)

	req.Type = "type"
	err = suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)

	req.Group = "group"
	err = suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)

	req.Name = "name"
	err = suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)

	req.TestSettings = map[string]*billingpb.PaymentMethodParams{}
	err = suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)

	req.ExternalId = ""
	req.ProductionSettings = map[string]*billingpb.PaymentMethodParams{"RUB": {Currency: "RUB"}}
	err = suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorPaymentSystem, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethod_OkWithActivate() {
	req := &billingpb.PaymentMethod{
		LimitsCurrency:     "EUR",
		PaymentSystemId:    primitive.NewObjectID().Hex(),
		IsActive:           true,
		ExternalId:         "externalId",
		Type:               "type",
		Group:              "group",
		Name:               "name",
		TestSettings:       map[string]*billingpb.PaymentMethodParams{},
		ProductionSettings: map[string]*billingpb.PaymentMethodParams{"RUB": {Currency: "RUB"}},
	}
	rsp := &billingpb.ChangePaymentMethodResponse{}
	paymentSystem := &mocks.PaymentSystemRepositoryInterface{}

	paymentSystem.On("GetById", mock2.Anything, req.PaymentSystemId).Return(&billingpb.PaymentSystem{}, nil)
	suite.service.paymentSystemRepository = paymentSystem

	err := suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethod_OkWithoutActivate() {
	req := &billingpb.PaymentMethod{
		LimitsCurrency:  "EUR",
		PaymentSystemId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.ChangePaymentMethodResponse{}
	paymentSystem := &mocks.PaymentSystemRepositoryInterface{}

	paymentSystem.On("GetById", mock2.Anything, req.PaymentSystemId).Return(&billingpb.PaymentSystem{}, nil)
	suite.service.paymentSystemRepository = paymentSystem

	err := suite.service.CreateOrUpdatePaymentMethod(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethodProductionSettings_ErrorPaymentMethod() {
	req := &billingpb.ChangePaymentMethodParamsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
		Params:          &billingpb.PaymentMethodParams{},
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(nil, errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.CreateOrUpdatePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorUnknownMethod, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethodProductionSettings_ErrorUpdate() {
	req := &billingpb.ChangePaymentMethodParamsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
		Params: &billingpb.PaymentMethodParams{
			Currency:       "RUB",
			TerminalId:     "ID",
			Secret:         "unit_test",
			SecretCallback: "unit_test_callback",
		},
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{LimitsCurrency: "EUR"}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.CreateOrUpdatePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), "not found", rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethodProductionSettings_Ok() {
	req := &billingpb.ChangePaymentMethodParamsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
		Params: &billingpb.PaymentMethodParams{
			Currency:       "RUB",
			TerminalId:     "ID",
			Secret:         "unit_test",
			SecretCallback: "unit_test_callback",
		},
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{LimitsCurrency: "EUR"}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.CreateOrUpdatePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentMethodProductionSettings_EmptyByPaymentMethod() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.GetPaymentMethodSettingsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(nil, errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.GetPaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), rsp.Params, 0)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentMethodProductionSettings_Ok() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.GetPaymentMethodSettingsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("EUR", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		ProductionSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "EUR",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
				Secret:             "secret",
				SecretCallback:     "secret_callback",
				TerminalId:         "terminal_id",
			},
		},
	}, nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.GetPaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), rsp.Params, 1)
	assert.Equal(suite.T(), "EUR", rsp.Params[0].Currency)
	assert.Equal(suite.T(), "secret", rsp.Params[0].Secret)
	assert.Equal(suite.T(), "secret_callback", rsp.Params[0].SecretCallback)
	assert.Equal(suite.T(), "terminal_id", rsp.Params[0].TerminalId)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodProductionSettings_ErrorByPaymentMethod() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(nil, errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorUnknownMethod, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodProductionSettings_ErrorNoSettings() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId:    primitive.NewObjectID().Hex(),
		CurrencyA3:         "EUR",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		ProductionSettings: map[string]*billingpb.PaymentMethodParams{
			key: {Currency: "RUB", Secret: "unit_test"},
		},
	}, nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorNotFoundProductionSettings, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodProductionSettings_ErrorUpdate() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId:    primitive.NewObjectID().Hex(),
		CurrencyA3:         "RUB",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		ProductionSettings: map[string]*billingpb.PaymentMethodParams{
			key: {Currency: "RUB", Secret: "unit_test"},
		},
	}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(errors.New("service unavailable"))
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), "service unavailable", rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodProductionSettings_Ok() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId:    primitive.NewObjectID().Hex(),
		CurrencyA3:         "RUB",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		ProductionSettings: map[string]*billingpb.PaymentMethodParams{
			key: {Currency: "RUB", Secret: "unit_test"},
		},
	}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethodTestSettings_ErrorPaymentMethod() {
	req := &billingpb.ChangePaymentMethodParamsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
		Params:          &billingpb.PaymentMethodParams{},
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(nil, errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.CreateOrUpdatePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorUnknownMethod, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethodTestSettings_ErrorUpdate() {
	req := &billingpb.ChangePaymentMethodParamsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
		Params: &billingpb.PaymentMethodParams{
			Currency:       "RUB",
			TerminalId:     "ID",
			Secret:         "unit_test",
			SecretCallback: "unit_test_callback",
		},
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{LimitsCurrency: "EUR"}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.CreateOrUpdatePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), "not found", rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_CreateOrUpdatePaymentMethodTestSettings_Ok() {
	req := &billingpb.ChangePaymentMethodParamsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
		Params: &billingpb.PaymentMethodParams{
			Currency:       "RUB",
			TerminalId:     "ID",
			Secret:         "unit_test",
			SecretCallback: "unit_test_callback",
		},
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{LimitsCurrency: "EUR"}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.CreateOrUpdatePaymentMethodProductionSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentMethodTestSettings_EmptyByPaymentMethod() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.GetPaymentMethodSettingsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(nil, errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.GetPaymentMethodTestSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), rsp.Params, 0)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_GetPaymentMethodTestSettings_Ok() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.GetPaymentMethodSettingsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("EUR", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		TestSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "EUR",
				Secret:             "secret",
				SecretCallback:     "secret_callback",
				TerminalId:         "terminal_id",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
			},
		},
	}, nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.GetPaymentMethodTestSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), rsp.Params, 1)
	assert.Equal(suite.T(), "EUR", rsp.Params[0].Currency)
	assert.Equal(suite.T(), "secret", rsp.Params[0].Secret)
	assert.Equal(suite.T(), "secret_callback", rsp.Params[0].SecretCallback)
	assert.Equal(suite.T(), "terminal_id", rsp.Params[0].TerminalId)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodTestSettings_ErrorByPaymentMethod() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(nil, errors.New("not found"))
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodTestSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorUnknownMethod, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodTestSettings_ErrorNoSettings() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId:    primitive.NewObjectID().Hex(),
		CurrencyA3:         "EUR",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		TestSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "RUB",
				Secret:             "unit_test",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
			},
		},
	}, nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodTestSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), paymentMethodErrorNotFoundProductionSettings, rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodTestSettings_ErrorUpdate() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId:    primitive.NewObjectID().Hex(),
		CurrencyA3:         "RUB",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		TestSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "RUB",
				Secret:             "unit_test",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
			},
		},
	}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(errors.New("service unavailable"))
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodTestSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), "service unavailable", rsp.Message)
}

func (suite *PaymentMethodTestSuite) TestPaymentMethod_DeletePaymentMethodTestSettings_Ok() {
	req := &billingpb.GetPaymentMethodSettingsRequest{
		PaymentMethodId:    primitive.NewObjectID().Hex(),
		CurrencyA3:         "RUB",
		MccCode:            billingpb.MccCodeLowRisk,
		OperatingCompanyId: suite.operatingCompany.Id,
	}
	rsp := &billingpb.ChangePaymentMethodParamsResponse{}
	method := &mocks.PaymentMethodRepositoryInterface{}

	key := helper.GetPaymentMethodKey("RUB", billingpb.MccCodeLowRisk, suite.operatingCompany.Id, "")
	method.On("GetById", mock2.Anything, req.PaymentMethodId).Return(&billingpb.PaymentMethod{
		LimitsCurrency: "EUR",
		TestSettings: map[string]*billingpb.PaymentMethodParams{
			key: {
				Currency:           "RUB",
				Secret:             "unit_test",
				MccCode:            billingpb.MccCodeLowRisk,
				OperatingCompanyId: suite.operatingCompany.Id,
				Brand:              []string{"VISA", "MASTERCARD"},
			},
		},
	}, nil)
	method.On("Update", mock2.Anything, mock2.Anything).Return(nil)
	suite.service.paymentMethodRepository = method

	err := suite.service.DeletePaymentMethodTestSettings(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}
