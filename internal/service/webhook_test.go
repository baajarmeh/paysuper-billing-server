package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/golang-migrate/migrate/v4"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/internal/payment_system"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	"github.com/paysuper/paysuper-proto/go/notifierpb"
	notifierpbMock "github.com/paysuper/paysuper-proto/go/notifierpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"strings"
	"testing"
)

type WebhookTestSuite struct {
	suite.Suite
	cfg     *config.Config
	service *Service
	cache   database.CacheInterface

	logObserver *zap.Logger
	zapRecorder *observer.ObservedLogs

	merchant                      *billingpb.Merchant
	project                       *billingpb.Project
	projectWithoutVirtualCurrency *billingpb.Project
	customer                      *billingpb.Customer
	products                      []*billingpb.Product
	keyProducts                   []*billingpb.KeyProduct

	request *billingpb.OrderCreateRequest
	order   *billingpb.Order
}

func Test_Webhook(t *testing.T) {
	suite.Run(t, new(WebhookTestSuite))
}

func (suite *WebhookTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}
	suite.cfg = cfg

	m, err := migrate.New("file://../../migrations/tests", cfg.MongoDsn)
	if err != nil {
		suite.FailNow("Migrate init failed", "%v", err)
	}

	err = m.Up()
	if err != nil && err.Error() != "no change" {
		suite.FailNow("Migrations failed", "%v", err)
	}

	db, err := mongodb.NewDatabase()
	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	redis := mocks.NewTestRedis()
	suite.cache, err = database.NewCacheRedis(redis, "cache")

	if err != nil {
		suite.FailNow("Cache redis initialize failed", "%v", err)
	}

	suite.service = NewBillingService(
		db,
		cfg,
		mocks.NewGeoIpServiceTestOk(),
		mocks.NewRepositoryServiceOk(),
		mocks.NewTaxServiceOkMock(),
		mocks.NewBrokerMockOk(),
		redis,
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

	suite.merchant, suite.project, _, _, suite.customer = HelperCreateEntitiesForTests(suite.Suite, suite.service)
	suite.products = CreateProductsForProject(suite.Suite, suite.service, suite.project, 3)
	suite.keyProducts = CreateKeyProductsForProject(suite.Suite, suite.service, suite.project, 3)
	suite.projectWithoutVirtualCurrency = HelperCreateProject(suite.Suite, suite.service, suite.merchant.Id, billingpb.VatPayerBuyer)

	suite.projectWithoutVirtualCurrency.VirtualCurrency = nil
	err = suite.service.project.Update(context.Background(), suite.projectWithoutVirtualCurrency)

	if err != nil {
		suite.FailNow("Remove virtual currency from project failed", "%v", err)
	}

	suite.project.WebhookTesting = nil
	err = suite.service.project.Update(context.Background(), suite.project)

	if err != nil {
		suite.FailNow("Remove webhook testing results from project failed", "%v", err)
	}

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)
	zap.ReplaceGlobals(suite.logObserver)

	suite.request = &billingpb.OrderCreateRequest{
		Type:        "virtual_currency",
		TestingCase: billingpb.TestCaseNonExistingUser,
		User: &billingpb.OrderUser{
			ExternalId: "unit_test",
			Address:    &billingpb.OrderBillingAddress{Country: "RU"},
		},
		Amount:    100,
		ProjectId: suite.project.Id,
	}
	suite.order = &billingpb.Order{
		Uuid: "254e3736-000f-5000-8000-178d1d80bf70",
		Project: &billingpb.ProjectOrder{
			UrlProcessPayment: "http://localhost/api/v1",
			SecretKey:         "secret",
			Status:            billingpb.ProjectStatusInProduction,
			MerchantId:        "254e3736-000f-5000-8000-178d1d80bf70",
		},
		User: &billingpb.OrderUser{
			Id:         "254e3736-000f-5000-8000-178d1d80bf70",
			Object:     "user",
			ExternalId: "254e3736-000f-5000-8000-178d1d80bf70",
			Name:       "Unit Test",
			Ip:         "127.0.0.1",
			Locale:     "ru",
		},
	}
}

func (suite *WebhookTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *WebhookTestSuite) Test_SendWebhook_VirtualCurrency_VerifyUser_Ok() {
	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotEmpty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_VirtualCurrency_Notification_Ok() {
	suite.request.TestingCase = billingpb.TestCaseIncorrectPayment

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotEmpty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Product_VerifyUser_Ok() {
	suite.request.Products = []string{suite.products[0].Id}
	suite.request.Type = pkg.OrderType_product

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotEmpty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Processor_ProcessProject_ResponseErrorMessage_Error() {
	projectRepositoryMock := &mocks.ProjectRepositoryInterface{}
	projectRepositoryMock.On("GetById", mock.Anything, mock.Anything).Return(nil, orderErrorProjectNotFound)
	suite.service.project = projectRepositoryMock

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorProjectNotFound, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Processor_ProcessVirtualCurrency_Error() {
	suite.request.ProjectId = suite.projectWithoutVirtualCurrency.Id

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorVirtualCurrencyNotFilled, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Processor_ProcessUserData_Error() {
	customerRepositoryMock := &mocks.CustomerRepositoryInterface{}
	customerRepositoryMock.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("some error"))
	customerRepositoryMock.On("Insert", mock.Anything, mock.Anything).
		Return(errors.New("some error"))
	suite.service.customerRepository = customerRepositoryMock

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorUnknown, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Processor_ProcessCurrency_Error() {
	suite.request.Type = pkg.OrderType_simple

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorCurrencyIsRequired, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_CheckedCurrencyIsEmpty_Error() {
	priceGroupRepositoryMock := &mocks.PriceGroupRepositoryInterface{}
	priceGroupRepositoryMock.On("GetByRegion", mock.Anything, mock.Anything).
		Return(
			&billingpb.PriceGroup{
				Id:            "254e3736-000f-5000-8000-178d1d80bf70",
				Currency:      "",
				Region:        "RUB",
				InflationRate: 10,
				Fraction:      10,
				IsActive:      true,
			},
			nil,
		)
	priceGroupRepositoryMock.On("GetById", mock.Anything, mock.Anything).
		Return(&billingpb.PriceGroup{Currency: ""}, nil)
	countryRepositoryMock := &mocks.CountryRepositoryInterface{}
	countryRepositoryMock.On("GetByIsoCodeA2", mock.Anything, mock.Anything).
		Return(&billingpb.Country{PriceGroupId: "254e3736-000f-5000-8000-178d1d80bf70"}, nil)

	suite.service.country = countryRepositoryMock
	suite.service.priceGroupRepository = priceGroupRepositoryMock

	suite.request.User.Address = &billingpb.OrderBillingAddress{Country: "RU"}

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorCurrencyIsRequired, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Processor_PrepareOrder_Error() {
	suite.request.UrlVerify = "http://localhost/success"

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorDynamicNotifyUrlsNotAllowed, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Broker_Publish_Error() {
	messages := suite.zapRecorder.All()
	assert.Empty(suite.T(), messages)

	suite.service.broker = mocks.NewBrokerMockError()
	suite.request.TestingCase = billingpb.TestCaseIncorrectPayment

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotEmpty(suite.T(), rsp.OrderId)

	messages = suite.zapRecorder.All()
	assert.NotEmpty(suite.T(), messages)
	assert.Equal(suite.T(), zap.ErrorLevel, messages[len(messages)-1].Level)
	assert.Equal(suite.T(), brokerPublicationFailed, messages[len(messages)-1].Message)
}

func (suite *WebhookTestSuite) Test_SendWebhook_Product_Processor_ProcessPaylinkProducts_PaylinkIdNotEmpty_Error() {
	messages := suite.zapRecorder.All()
	assert.Empty(suite.T(), messages)

	priceGroupRepositoryMock := &mocks.PriceGroupRepositoryInterface{}
	priceGroupRepositoryMock.On("GetByRegion", mock.Anything, mock.Anything).
		Return(
			&billingpb.PriceGroup{
				Id:            "254e3736-000f-5000-8000-178d1d80bf70",
				Currency:      "AUD",
				Region:        "",
				InflationRate: 10,
				Fraction:      10,
				IsActive:      true,
			},
			nil,
		)
	priceGroupRepositoryMock.On("GetById", mock.Anything, mock.Anything).
		Return(&billingpb.PriceGroup{Currency: ""}, nil)

	suite.request.Products = []string{suite.products[0].Id}
	suite.request.Type = pkg.OrderType_product
	suite.request.PrivateMetadata = map[string]string{"PaylinkId": "254e3736-000f-5000-8000-178d1d80bf70"}
	suite.service.priceGroupRepository = priceGroupRepositoryMock
	suite.service.centrifugoDashboard = newCentrifugo(suite.cfg.CentrifugoDashboard, payment_system.NewClientStatusOk())

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), productNoPriceInCurrencyError, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)

	messages = suite.zapRecorder.All()
	assert.NotEmpty(suite.T(), messages)

	jRequest, err := json.Marshal(suite.request)
	assert.NoError(suite.T(), err)
	mapRequest := make(map[string]interface{})
	err = json.Unmarshal(jRequest, &mapRequest)

	lastMessage := messages[len(messages)-1]
	assert.Equal(suite.T(), zap.InfoLevel, lastMessage.Level)
	assert.Equal(suite.T(), "/dashboard/api", lastMessage.Message)
	msg := make(map[string]interface{})
	err = json.Unmarshal(messages[0].Context[1].Interface.([]byte), &msg)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "params")
	params := msg["params"].(map[string]interface{})
	assert.Contains(suite.T(), params, "data")
	assert.Contains(suite.T(), params, "channel")
	assert.Equal(suite.T(), centrifugoChannel, params["channel"])
	data := params["data"].(map[string]interface{})
	assert.Equal(suite.T(), "error", data["event"])
	assert.Equal(suite.T(), "254e3736-000f-5000-8000-178d1d80bf70", data["paylinkId"])
	assert.Equal(suite.T(), "Invalid paylink", data["message"])
	assert.Equal(suite.T(), mapRequest, data["request"])
	assert.Nil(suite.T(), data["order"])
	assert.Equal(suite.T(), strings.ToLower(productNoPriceInCurrencyError.Error()), strings.ToLower(data["error"].(string)))
}

func (suite *WebhookTestSuite) Test_SendWebhook_Product_Processor_ProcessPaylinkProducts_Error() {
	suite.request.Products = []string{}
	suite.request.Type = pkg.OrderType_product

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorProductsEmpty, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_KeyProduct_Ok() {
	suite.request.Products = []string{suite.keyProducts[0].Id}
	suite.request.Type = pkg.OrderType_key

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotEmpty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_SendWebhook_KeyProduct_Processor_ProcessPaylinkKeyProducts_Error() {
	messages := suite.zapRecorder.All()
	assert.Empty(suite.T(), messages)

	suite.request.Products = []string{}
	suite.request.Type = pkg.OrderType_key
	suite.request.PrivateMetadata = map[string]string{"PaylinkId": "254e3736-000f-5000-8000-178d1d80bf70"}
	suite.service.centrifugoDashboard = newCentrifugo(suite.cfg.CentrifugoDashboard, payment_system.NewClientStatusOk())

	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorProductsEmpty, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)

	messages = suite.zapRecorder.All()
	assert.NotEmpty(suite.T(), messages)

	jRequest, err := json.Marshal(suite.request)
	assert.NoError(suite.T(), err)
	mapRequest := make(map[string]interface{})
	err = json.Unmarshal(jRequest, &mapRequest)

	lastMessage := messages[len(messages)-1]
	assert.Equal(suite.T(), zap.InfoLevel, lastMessage.Level)
	assert.Equal(suite.T(), "/dashboard/api", lastMessage.Message)
	msg := make(map[string]interface{})
	err = json.Unmarshal(messages[0].Context[1].Interface.([]byte), &msg)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "params")
	params := msg["params"].(map[string]interface{})
	assert.Contains(suite.T(), params, "data")
	assert.Contains(suite.T(), params, "channel")
	assert.Equal(suite.T(), centrifugoChannel, params["channel"])
	data := params["data"].(map[string]interface{})
	assert.Equal(suite.T(), "error", data["event"])
	assert.Equal(suite.T(), "254e3736-000f-5000-8000-178d1d80bf70", data["paylinkId"])
	assert.Equal(suite.T(), "Invalid paylink", data["message"])
	assert.Equal(suite.T(), mapRequest, data["request"])
	assert.Nil(suite.T(), data["order"])
	assert.Equal(suite.T(), strings.ToLower(orderErrorProductsEmpty.Error()), strings.ToLower(data["error"].(string)))
}

func (suite *WebhookTestSuite) Test_SendWebhook_UnknownWebhookType_Error() {
	suite.request.Type = "unknown"
	rsp := &billingpb.SendWebhookToMerchantResponse{}
	err := suite.service.SendWebhookToMerchant(ctx, suite.request, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), webhookTypeIncorrect, rsp.Message)
	assert.Empty(suite.T(), rsp.OrderId)
}

func (suite *WebhookTestSuite) Test_NotifyWebhookTestResults_Ok() {
	req := &billingpb.NotifyWebhookTestResultsRequest{
		ProjectId: suite.project.Id,
		IsPassed:  true,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	cases := []string{
		pkg.TestCaseNonExistingUser,
		pkg.TestCaseExistingUser,
		pkg.TestCaseCorrectPayment,
		pkg.TestCaseIncorrectPayment,
	}
	types := []string{pkg.OrderType_product, pkg.OrderType_key, pkg.OrderTypeVirtualCurrency}

	for _, val := range types {
		for _, val1 := range cases {
			req.Type = val
			req.TestCase = val1
			err := suite.service.NotifyWebhookTestResults(context.TODO(), req, rsp)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)

			project, err := suite.service.project.GetById(context.TODO(), req.ProjectId)
			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), project)
			assert.NotNil(suite.T(), project.WebhookTesting)
			project.WebhookTesting = nil
			err = suite.service.project.Update(context.Background(), project)
			assert.NoError(suite.T(), err)
		}
	}
}

func (suite *WebhookTestSuite) Test_NotifyWebhookTestResults_ProjectNotFound_Error() {
	req := &billingpb.NotifyWebhookTestResultsRequest{
		ProjectId: "ffffffffffffffffffffffff",
		IsPassed:  true,
		Type:      pkg.OrderType_product,
		TestCase:  pkg.TestCaseNonExistingUser,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.NotifyWebhookTestResults(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), projectErrorUnknown, rsp.Message)
}

func (suite *WebhookTestSuite) Test_NotifyWebhookTestResults_IncorrectType_Error() {
	req := &billingpb.NotifyWebhookTestResultsRequest{
		ProjectId: suite.project.Id,
		IsPassed:  true,
		Type:      "unknown",
		TestCase:  pkg.TestCaseNonExistingUser,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.NotifyWebhookTestResults(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), webhookTypeIncorrect, rsp.Message)
}

func (suite *WebhookTestSuite) Test_NotifyWebhookTestResults_ProjectUpdate_Error() {
	projectRepositoryMock := &mocks.ProjectRepositoryInterface{}
	projectRepositoryMock.On("GetById", mock.Anything, mock.Anything).
		Return(suite.project, nil)
	projectRepositoryMock.On("Update", mock.Anything, mock.Anything).
		Return(errors.New("ProjectUpdate"))
	suite.service.project = projectRepositoryMock

	req := &billingpb.NotifyWebhookTestResultsRequest{
		ProjectId: suite.project.Id,
		IsPassed:  true,
		Type:      pkg.OrderType_product,
		TestCase:  pkg.TestCaseNonExistingUser,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.NotifyWebhookTestResults(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), projectErrorUnknown, rsp.Message)
}

func (suite *WebhookTestSuite) Test_WebhookCheckUser_Ok() {
	err := suite.service.webhookCheckUser(suite.order)
	assert.NoError(suite.T(), err)
}

func (suite *WebhookTestSuite) Test_WebhookCheckUser_NotifierSystemError() {
	messages := suite.zapRecorder.All()
	assert.Empty(suite.T(), messages)

	notifierServiceMock := &notifierpbMock.NotifierService{}
	notifierServiceMock.On("CheckUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("NotifierService_CheckUser"))
	suite.service.notifier = notifierServiceMock
	err := suite.service.webhookCheckUser(suite.order)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorMerchantUserAccountNotChecked, err)

	messages = suite.zapRecorder.All()
	assert.NotEmpty(suite.T(), messages)
	assert.Equal(suite.T(), zap.ErrorLevel, messages[0].Level)
	assert.Equal(suite.T(), pkg.ErrorGrpcServiceCallFailed, messages[0].Message)
}

func (suite *WebhookTestSuite) Test_WebhookCheckUser_NotifierResultError() {
	messages := suite.zapRecorder.All()
	assert.Empty(suite.T(), messages)

	notifierServiceMock := &notifierpbMock.NotifierService{}
	notifierServiceMock.On("CheckUser", mock.Anything, mock.Anything, mock.Anything).
		Return(&notifierpb.CheckUserResponse{Status: billingpb.ResponseStatusNotFound}, nil)
	suite.service.notifier = notifierServiceMock
	err := suite.service.webhookCheckUser(suite.order)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorMerchantUserAccountNotChecked, err)

	messages = suite.zapRecorder.All()
	assert.NotEmpty(suite.T(), messages)
	assert.Equal(suite.T(), zap.ErrorLevel, messages[0].Level)
	assert.Equal(suite.T(), pkg.ErrorUserCheckFailed, messages[0].Message)
}
