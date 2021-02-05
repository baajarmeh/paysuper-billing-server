package service

import (
	"context"
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	recurringMocks "github.com/paysuper/paysuper-proto/go/recurringpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type RecurringTestSuite struct {
	suite.Suite
	service *Service
}

func Test_Recurring(t *testing.T) {
	suite.Run(t, new(RecurringTestSuite))
}

func (suite *RecurringTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

	db, err := mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	redisdb := mocks.NewTestRedis()
	cache, err := database.NewCacheRedis(redisdb, "cache")

	if err != nil {
		suite.FailNow("Cache redis initialize failed", "%v", err)
	}

	casbin := &casbinMocks.CasbinService{}

	suite.service = NewBillingService(
		db,
		cfg,
		mocks.NewGeoIpServiceTestOk(),
		mocks.NewRepositoryServiceOk(),
		&mocks.TaxServiceOkMock{},
		mocks.NewBrokerMockOk(),
		nil,
		cache,
		mocks.NewCurrencyServiceMockOk(),
		mocks.NewDocumentSignerMockOk(),
		&reportingMocks.ReporterService{},
		mocks.NewFormatterOK(),
		mocks.NewBrokerMockOk(),
		casbin,
		mocks.NewNotifierOk(),
		mocks.NewBrokerMockOk(),
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}
}

func (suite *RecurringTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_Ok() {
	customer := &BrowserCookieCustomer{
		VirtualCustomerId: primitive.NewObjectID().Hex(),
		Ip:                "127.0.0.1",
		AcceptLanguage:    "fr-CA",
		UserAgent:         "windows",
		SessionCount:      0,
	}
	cookie, err := suite.service.generateBrowserCookie(customer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: cookie,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_IncorrectCookie_Error() {
	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: primitive.NewObjectID().Hex(),
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), recurringErrorIncorrectCookie, rsp.Message)
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_DontHaveCustomerId_Error() {
	customer := &BrowserCookieCustomer{
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(customer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: cookie,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), recurringCustomerNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_RealCustomer_Ok() {
	project := &billingpb.Project{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
	}
	req0 := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: project.Id,
			Amount:    100,
			Currency:  "USD",
			Type:      pkg.OrderType_simple,
		},
	}
	customer, err := suite.service.createCustomer(context.TODO(), req0, project)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)

	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     customer.Id,
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: cookie,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_RealCustomerNotFound_Error() {
	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     primitive.NewObjectID().Hex(),
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: cookie,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), recurringCustomerNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_RecurringServiceSystem_Error() {
	browserCustomer := &BrowserCookieCustomer{
		VirtualCustomerId: primitive.NewObjectID().Hex(),
		Ip:                "127.0.0.1",
		AcceptLanguage:    "fr-CA",
		UserAgent:         "windows",
		SessionCount:      0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	suite.service.rep = mocks.NewRepositoryServiceError()

	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: cookie,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, rsp.Message)
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_RecurringServiceResult_Error() {
	browserCustomer := &BrowserCookieCustomer{
		VirtualCustomerId: primitive.NewObjectID().Hex(),
		Ip:                "127.0.0.1",
		AcceptLanguage:    "fr-CA",
		UserAgent:         "windows",
		SessionCount:      0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	suite.service.rep = mocks.NewRepositoryServiceEmpty()

	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: cookie,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), recurringSavedCardNotFount, rsp.Message)
}

func (suite *RecurringTestSuite) TestRecurring_DeleteSavedCard_RecurringServiceResultSystemError_Error() {
	browserCustomer := &BrowserCookieCustomer{
		VirtualCustomerId: "ffffffffffffffffffffffff",
		Ip:                "127.0.0.1",
		AcceptLanguage:    "fr-CA",
		UserAgent:         "windows",
		SessionCount:      0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	suite.service.rep = mocks.NewRepositoryServiceEmpty()

	req := &billingpb.DeleteSavedCardRequest{
		Id:     primitive.NewObjectID().Hex(),
		Cookie: cookie,
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteSavedCard(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_WithoutCookie_Ok() {
	var (
		merchantId = "customer_id"
		orderId    = "order_id"
		psId       = "payment_system_id"
		psHandler  = "payment_system_handler"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	recurring.On("DeleteSubscription", mock.Anything, subscription).Return(&recurringpb.DeleteSubscriptionResponse{
		Status: billingpb.ResponseStatusOk,
	}, nil)
	suite.service.rep = recurring

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetById", mock.Anything, orderId).Return(order, nil)
	suite.service.orderRepository = orderRepository

	psRepository := &mocks.PaymentSystemRepositoryInterface{}
	psRepository.On("GetById", mock.Anything, psId).Return(&billingpb.PaymentSystem{
		Handler: psHandler,
	}, nil)
	suite.service.paymentSystemRepository = psRepository

	paymentSystem := &mocks.PaymentSystemInterface{}
	paymentSystem.On("DeleteRecurringSubscription", order, subscription).Return(nil)

	gatewayManagerMock := &mocks.PaymentSystemManagerInterface{}
	gatewayManagerMock.On("GetGateway", psHandler).Return(paymentSystem, nil)
	suite.service.paymentSystemGateway = gatewayManagerMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         order.RecurringId,
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_WithCookie_Ok() {
	var (
		customerId = "customer_id"
		orderId    = "order_id"
		psId       = "payment_system_id"
		psHandler  = "payment_system_handler"
	)

	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     customerId,
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		CustomerId: customerId,
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	recurring.On("DeleteSubscription", mock.Anything, subscription).Return(&recurringpb.DeleteSubscriptionResponse{
		Status: billingpb.ResponseStatusOk,
	}, nil)
	suite.service.rep = recurring

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetById", mock.Anything, orderId).Return(order, nil)
	suite.service.orderRepository = orderRepository

	psRepository := &mocks.PaymentSystemRepositoryInterface{}
	psRepository.On("GetById", mock.Anything, psId).Return(&billingpb.PaymentSystem{
		Handler: psHandler,
	}, nil)
	suite.service.paymentSystemRepository = psRepository

	paymentSystem := &mocks.PaymentSystemInterface{}
	paymentSystem.On("DeleteRecurringSubscription", order, subscription).Return(nil)

	gatewayManagerMock := &mocks.PaymentSystemManagerInterface{}
	gatewayManagerMock.On("GetGateway", psHandler).Return(paymentSystem, nil)
	suite.service.paymentSystemGateway = gatewayManagerMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:     order.RecurringId,
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_BadCookie_Error() {
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:     "recurring_id",
		Cookie: "cookie",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringCustomerNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_NoCustomerOnCookie_Error() {
	browserCustomer := &BrowserCookieCustomer{
		VirtualCustomerId: "customerId",
		Ip:                "127.0.0.1",
		AcceptLanguage:    "fr-CA",
		UserAgent:         "windows",
		SessionCount:      0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:     "recurring_id",
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_SubscriptionNotFound_Error() {
	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status: billingpb.ResponseStatusNotFound,
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id: "recurring_id",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorRecurringSubscriptionNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_AccessDenyByEmptyIdentifiers_Error() {
	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{MerchantId: "merchant_id2"},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id: "recurring_id",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_AccessDenyByMerchant_Error() {
	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{MerchantId: "merchant_id2"},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         "recurring_id",
		MerchantId: "merchant_id",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_AccessDenyByCustomer_Error() {
	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     "customer_id",
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{CustomerId: "customer_id2"},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:     "recurring_id",
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_OrderNotFound_Error() {
	var (
		merchantId = "merchant_id"
		orderId    = "order_id"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetById", mock.Anything, orderId).Return(nil, errors.New("notfound"))
	suite.service.orderRepository = orderRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         "recurring_id",
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_PaymentSystemNotFound_Error() {
	var (
		merchantId = "merchant_id"
		orderId    = "order_id"
		psId       = "payment_system_id"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetById", mock.Anything, orderId).Return(order, nil)
	suite.service.orderRepository = orderRepository

	psRepository := &mocks.PaymentSystemRepositoryInterface{}
	psRepository.On("GetById", mock.Anything, psId).Return(nil, errors.New("notfound"))
	suite.service.paymentSystemRepository = psRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         "recurring_id",
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorPaymentSystemInactive, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_PaymentSystemGatewayNotFound_Error() {
	var (
		merchantId = "merchant_id"
		orderId    = "order_id"
		psId       = "payment_system_id"
		psHandler  = "payment_system_handler"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetById", mock.Anything, orderId).Return(order, nil)
	suite.service.orderRepository = orderRepository

	psRepository := &mocks.PaymentSystemRepositoryInterface{}
	psRepository.On("GetById", mock.Anything, psId).Return(&billingpb.PaymentSystem{
		Handler: psHandler,
	}, nil)
	suite.service.paymentSystemRepository = psRepository

	paymentSystem := &mocks.PaymentSystemInterface{}
	paymentSystem.On("DeleteRecurringSubscription", order, subscription).Return(nil)

	gatewayManagerMock := &mocks.PaymentSystemManagerInterface{}
	gatewayManagerMock.On("GetGateway", psHandler).Return(nil, errors.New("notfound"))
	suite.service.paymentSystemGateway = gatewayManagerMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         "recurring_id",
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), orderErrorPaymentSystemInactive, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_DeleteSubscriptionOnPaymentSystem_Error() {
	var (
		merchantId = "merchant_id"
		orderId    = "order_id"
		psId       = "payment_system_id"
		psHandler  = "payment_system_handler"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetById", mock.Anything, orderId).Return(order, nil)
	suite.service.orderRepository = orderRepository

	psRepository := &mocks.PaymentSystemRepositoryInterface{}
	psRepository.On("GetById", mock.Anything, psId).Return(&billingpb.PaymentSystem{
		Handler: psHandler,
	}, nil)
	suite.service.paymentSystemRepository = psRepository

	paymentSystem := &mocks.PaymentSystemInterface{}
	paymentSystem.On("DeleteRecurringSubscription", order, subscription).Return(errors.New("error"))

	gatewayManagerMock := &mocks.PaymentSystemManagerInterface{}
	gatewayManagerMock.On("GetGateway", psHandler).Return(paymentSystem, nil)
	suite.service.paymentSystemGateway = gatewayManagerMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         order.RecurringId,
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorDeleteSubscription, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_DeleteFromRepository_Error() {
	var (
		merchantId = "merchant_id"
		orderId    = "order_id"
		psId       = "payment_system_id"
		psHandler  = "payment_system_handler"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	recurring.On("DeleteSubscription", mock.Anything, subscription).Return(&recurringpb.DeleteSubscriptionResponse{
		Status: billingpb.ResponseStatusSystemError,
	}, nil)
	suite.service.rep = recurring

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetById", mock.Anything, orderId).Return(order, nil)
	suite.service.orderRepository = orderRepository

	psRepository := &mocks.PaymentSystemRepositoryInterface{}
	psRepository.On("GetById", mock.Anything, psId).Return(&billingpb.PaymentSystem{
		Handler: psHandler,
	}, nil)
	suite.service.paymentSystemRepository = psRepository

	paymentSystem := &mocks.PaymentSystemInterface{}
	paymentSystem.On("DeleteRecurringSubscription", order, subscription).Return(nil)

	gatewayManagerMock := &mocks.PaymentSystemManagerInterface{}
	gatewayManagerMock.On("GetGateway", psHandler).Return(paymentSystem, nil)
	suite.service.paymentSystemGateway = gatewayManagerMock

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         order.RecurringId,
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorDeleteSubscription, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_WithoutCookie_Ok() {
	var (
		merchantId = "merchant_id"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:         order.RecurringId,
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_WithCookie_Ok() {
	var (
		customerId = "customer_id"
	)

	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     customerId,
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		CustomerId: customerId,
	}
	order := &billingpb.Order{}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err = suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:     order.RecurringId,
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_BadCookie_Error() {
	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:     "recurring_id",
		Cookie: "cookie",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringCustomerNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_NoCustomerOnCookie_Error() {
	browserCustomer := &BrowserCookieCustomer{
		VirtualCustomerId: "customerId",
		Ip:                "127.0.0.1",
		AcceptLanguage:    "fr-CA",
		UserAgent:         "windows",
		SessionCount:      0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err = suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:     "recurring_id",
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_SubscriptionNotFound_Error() {
	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status: billingpb.ResponseStatusNotFound,
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:         "recurring_id",
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), recurringErrorSubscriptionNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_GetSubscription_Error() {
	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(nil, errors.New("err"))
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:         "recurring_id",
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_AccessDenyByMerchant_Error() {
	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{MerchantId: "merchant_id2"},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:         "recurring_id",
		MerchantId: "merchant_id",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_AccessDenyByEmptyIdentifiers_Error() {
	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{MerchantId: "merchant_id2"},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id: "recurring_id",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_AccessDenyByCustomer_Error() {
	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     "customer_id",
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: &recurringpb.Subscription{CustomerId: "customer_id2"},
	}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err = suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:     "recurring_id",
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_GetOrders_Error() {
	var (
		merchantId = "merchant_id"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	orderViewRepository := &mocks.OrderViewRepositoryInterface{}
	orderViewRepository.On("GetManyBy", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("error"))
	suite.service.orderViewRepository = orderViewRepository

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:         order.RecurringId,
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, rsp.Message)
}

func (suite *RecurringTestSuite) TestGetSubscriptionOrders_HasOrders_Ok() {
	var (
		merchantId = "merchant_id"
	)

	subscription := &recurringpb.Subscription{
		OrderId:    "order_id",
		MerchantId: merchantId,
	}
	order := &billingpb.Order{}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("GetSubscription", mock.Anything, mock.Anything).Return(&recurringpb.GetSubscriptionResponse{
		Status:       billingpb.ResponseStatusOk,
		Subscription: subscription,
	}, nil)
	suite.service.rep = recurring

	orderViewRepository := &mocks.OrderViewRepositoryInterface{}
	orderViewRepository.On("GetManyBy", mock.Anything, mock.Anything, mock.Anything).
		Return([]*billingpb.OrderViewPrivate{{
			Uuid:            "order_uuid",
			OrderCharge:     &billingpb.OrderViewMoney{Amount: 1, Currency: "USD"},
			TransactionDate: ptypes.TimestampNow(),
			PaymentMethod: &billingpb.PaymentMethodOrder{
				Card: &billingpb.PaymentMethodCard{Masked: "1234"},
			},
			Items: []*billingpb.OrderItem{{
				Name: "item_name",
			}},
		}}, nil)
	orderViewRepository.On("GetCountBy", mock.Anything, mock.Anything).
		Return(int64(1), nil)
	suite.service.orderViewRepository = orderViewRepository

	rsp := &billingpb.GetSubscriptionOrdersResponse{}
	err := suite.service.GetSubscriptionOrders(context.Background(), &billingpb.GetSubscriptionOrdersRequest{
		Id:         order.RecurringId,
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestFindSubscriptions_WithoutCookie_Ok() {
	var (
		merchantId = "merchant_id"
	)

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("FindSubscriptions", mock.Anything, mock.Anything).
		Return(&recurringpb.FindSubscriptionsResponse{
			List:  nil,
			Count: 0,
		}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.FindSubscriptionsResponse{}
	err := suite.service.FindSubscriptions(context.Background(), &billingpb.FindSubscriptionsRequest{
		MerchantId: merchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestFindSubscriptions_WithCookie_Ok() {
	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     "customer_id",
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("FindSubscriptions", mock.Anything, mock.Anything).
		Return(&recurringpb.FindSubscriptionsResponse{
			List:  nil,
			Count: 0,
		}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.FindSubscriptionsResponse{}
	err = suite.service.FindSubscriptions(context.Background(), &billingpb.FindSubscriptionsRequest{
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestFindSubscriptions_BadCookie_Error() {
	rsp := &billingpb.FindSubscriptionsResponse{}
	err := suite.service.FindSubscriptions(context.Background(), &billingpb.FindSubscriptionsRequest{
		Cookie: "cookie",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringCustomerNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestFindSubscriptions_NoCustomerOnCookie_Error() {
	browserCustomer := &BrowserCookieCustomer{
		VirtualCustomerId: "customerId",
		Ip:                "127.0.0.1",
		AcceptLanguage:    "fr-CA",
		UserAgent:         "windows",
		SessionCount:      0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	rsp := &billingpb.FindSubscriptionsResponse{}
	err = suite.service.FindSubscriptions(context.Background(), &billingpb.FindSubscriptionsRequest{
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestFindSubscriptions_AccessDenyByEmptyIdentifiers_Error() {
	rsp := &billingpb.FindSubscriptionsResponse{}
	err := suite.service.FindSubscriptions(context.Background(), &billingpb.FindSubscriptionsRequest{}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestFindSubscriptions_RemapRequestForFindSubscription_Ok() {
	var (
		customerId = "customer_id"
	)

	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     customerId,
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	req := &billingpb.FindSubscriptionsRequest{
		MerchantId:  "merchant_id",
		Cookie:      cookie,
		QuickFilter: "quick_filter",
		Offset:      50,
		Limit:       100,
	}

	recurring := &recurringMocks.RepositoryService{}
	recurring.On("FindSubscriptions", mock.Anything, mock.MatchedBy(func(input *recurringpb.FindSubscriptionsRequest) bool {
		return input.MerchantId == req.MerchantId &&
			input.CustomerId == customerId &&
			input.QuickFilter == req.QuickFilter &&
			input.Limit == req.Limit &&
			input.Offset == req.Offset
	})).
		Return(&recurringpb.FindSubscriptionsResponse{
			List:  nil,
			Count: 0,
		}, nil)
	suite.service.rep = recurring

	rsp := &billingpb.FindSubscriptionsResponse{}
	err = suite.service.FindSubscriptions(context.Background(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestMapRecurringToBilling_Ok() {
	subscription := &recurringpb.Subscription{
		Id:            "Id",
		CustomerId:    "CustomerId",
		CustomerInfo:  &recurringpb.CustomerInfo{Email: "Email"},
		Period:        "Period",
		MerchantId:    "MerchantId",
		ProjectId:     "ProjectId",
		Amount:        1,
		TotalAmount:   2,
		Currency:      "Currency",
		IsActive:      true,
		MaskedPan:     "MaskedPan",
		ExpireAt:      ptypes.TimestampNow(),
		CreatedAt:     ptypes.TimestampNow(),
		LastPaymentAt: ptypes.TimestampNow(),
		ProjectName:   map[string]string{"en": "ProjectName"},
	}

	billingSubscription := suite.service.mapRecurringToBilling(subscription)
	assert.Equal(suite.T(), subscription.Id, billingSubscription.Id)
	assert.Equal(suite.T(), subscription.CustomerId, billingSubscription.CustomerId)
	assert.Equal(suite.T(), subscription.CustomerInfo.Email, billingSubscription.CustomerEmail)
	assert.Equal(suite.T(), subscription.Period, billingSubscription.Period)
	assert.Equal(suite.T(), subscription.MerchantId, billingSubscription.MerchantId)
	assert.Equal(suite.T(), subscription.ProjectId, billingSubscription.ProjectId)
	assert.Equal(suite.T(), subscription.Amount, billingSubscription.Amount)
	assert.Equal(suite.T(), subscription.TotalAmount, billingSubscription.TotalAmount)
	assert.Equal(suite.T(), subscription.Currency, billingSubscription.Currency)
	assert.Equal(suite.T(), subscription.IsActive, billingSubscription.IsActive)
	assert.Equal(suite.T(), subscription.MaskedPan, billingSubscription.MaskedPan)
	assert.Equal(suite.T(), subscription.ExpireAt, billingSubscription.ExpireAt)
	assert.Equal(suite.T(), subscription.CreatedAt, billingSubscription.CreatedAt)
	assert.Equal(suite.T(), subscription.LastPaymentAt, billingSubscription.LastPaymentAt)
	assert.Equal(suite.T(), subscription.ProjectName, billingSubscription.ProjectName)
}

func (suite *RecurringTestSuite) TestPlanPermissionWithInvalidMerchant() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(nil, errors.New("error"))
	suite.service.merchantRepository = merchantRep

	res := &billingpb.AddRecurringPlanResponse{}
	err := suite.service.AddRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, res.Status)
	assert.Equal(suite.T(), errorMerchantNotFound, res.Message)
}

func (suite *RecurringTestSuite) TestPlanPermissionWithInvalidProject() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(nil, errors.New("error"))
	suite.service.project = projectRep

	res := &billingpb.AddRecurringPlanResponse{}
	err := suite.service.AddRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, res.Status)
	assert.Equal(suite.T(), recurringErrorProjectNotFound, res.Message)
}

func (suite *RecurringTestSuite) TestPlanPermissionWithProjectDontOwnMerchant() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: primitive.NewObjectID().Hex()}, nil)
	suite.service.project = projectRep

	res := &billingpb.AddRecurringPlanResponse{}
	err := suite.service.AddRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, res.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, res.Message)
}

func (suite *RecurringTestSuite) TestValidateRecurringPlanInvalidChargePeriodMinute() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Type:  recurringpb.RecurringPeriodMinute,
				Value: 61,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	err := suite.service.validateRecurringPlanRequest(req)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), recurringErrorInvalidPeriod, err)
}

func (suite *RecurringTestSuite) TestValidateRecurringPlanInvalidChargePeriodDay() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Type:  recurringpb.RecurringPeriodDay,
				Value: 366,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	err := suite.service.validateRecurringPlanRequest(req)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), recurringErrorInvalidPeriod, err)
}

func (suite *RecurringTestSuite) TestValidateRecurringPlanInvalidChargePeriodWeek() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Type:  recurringpb.RecurringPeriodWeek,
				Value: 53,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	err := suite.service.validateRecurringPlanRequest(req)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), recurringErrorInvalidPeriod, err)
}

func (suite *RecurringTestSuite) TestValidateRecurringPlanInvalidChargePeriodMonth() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Type:  recurringpb.RecurringPeriodMonth,
				Value: 13,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	err := suite.service.validateRecurringPlanRequest(req)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), recurringErrorInvalidPeriod, err)
}

func (suite *RecurringTestSuite) TestValidateRecurringPlanInvalidChargePeriodYear() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Type:  recurringpb.RecurringPeriodYear,
				Value: 2,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	err := suite.service.validateRecurringPlanRequest(req)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), recurringErrorInvalidPeriod, err)
}

func (suite *RecurringTestSuite) TestValidateRecurringPlanInvalidChargePeriodEmpty() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 0,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	err := suite.service.validateRecurringPlanRequest(req)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), recurringErrorInvalidPeriod, err)
}

func (suite *RecurringTestSuite) TestAddRecurringPlanError() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 1,
				Type:  recurringpb.RecurringPeriodDay,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("Insert", mock.Anything, req).Return(errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.AddRecurringPlanResponse{}
	err := suite.service.AddRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanCreate, res.Message)
}

func (suite *RecurringTestSuite) TestAddRecurringPlanWithEmptyStatusOk() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 1,
				Type:  recurringpb.RecurringPeriodDay,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("Insert", mock.Anything, req).Return(nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.AddRecurringPlanResponse{}
	err := suite.service.AddRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), req, res.Item)
	assert.NotEmpty(suite.T(), res.Item.Id)
	assert.Equal(suite.T(), pkg.RecurringPlanStatusDisabled, res.Item.Status)
}

func (suite *RecurringTestSuite) TestAddRecurringPlanOk() {
	req := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 1,
				Type:  recurringpb.RecurringPeriodDay,
			},
		},
		Status: pkg.RecurringPlanStatusActive,
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("Insert", mock.Anything, req).Return(nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.AddRecurringPlanResponse{}
	err := suite.service.AddRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), req, res.Item)
	assert.NotEmpty(suite.T(), res.Item.Id)
	assert.Equal(suite.T(), pkg.RecurringPlanStatusActive, res.Item.Status)
}

func (suite *RecurringTestSuite) TestUpdateRecurringPlanUnableGetById() {
	req := &billingpb.RecurringPlan{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 1,
				Type:  recurringpb.RecurringPeriodDay,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.Id).Return(nil, errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.UpdateRecurringPlanResponse{}
	err := suite.service.UpdateRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanNotFound, res.Message)
}

func (suite *RecurringTestSuite) TestUpdateRecurringPlanError() {
	req := &billingpb.RecurringPlan{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 1,
				Type:  recurringpb.RecurringPeriodDay,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.Id).Return(req, nil)
	planRep.On("Update", mock.Anything, req).Return(errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.UpdateRecurringPlanResponse{}
	err := suite.service.UpdateRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanUpdate, res.Message)
}

func (suite *RecurringTestSuite) TestUpdateRecurringPlanOk() {
	req := &billingpb.RecurringPlan{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 1,
				Type:  recurringpb.RecurringPeriodDay,
			},
		},
		Name:        map[string]string{"en": "en"},
		Description: map[string]string{"en": "en"},
		Tags:        []string{"tag"},
		Status:      pkg.RecurringPlanStatusActive,
		GroupId:     "group",
		ExternalId:  "ext",
		Expiration: &billingpb.RecurringPlanPeriod{
			Value: 1,
			Type:  recurringpb.RecurringPeriodDay,
		},
		Trial: &billingpb.RecurringPlanPeriod{
			Value: 1,
			Type:  recurringpb.RecurringPeriodDay,
		},
		GracePeriod: &billingpb.RecurringPlanPeriod{
			Value: 1,
			Type:  recurringpb.RecurringPeriodDay,
		},
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.Id,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
		Name:        map[string]string{"en": "ru"},
		Description: map[string]string{"en": "ru"},
		Tags:        []string{"tag2"},
		Status:      pkg.RecurringPlanStatusDisabled,
		GroupId:     "group2",
		ExternalId:  "ext2",
		Expiration: &billingpb.RecurringPlanPeriod{
			Value: 3,
			Type:  recurringpb.RecurringPeriodWeek,
		},
		Trial: &billingpb.RecurringPlanPeriod{
			Value: 4,
			Type:  recurringpb.RecurringPeriodMonth,
		},
		GracePeriod: &billingpb.RecurringPlanPeriod{
			Value: 5,
			Type:  recurringpb.RecurringPeriodYear,
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.Id).Return(plan, nil)
	planRep.On("Update", mock.Anything, req).Return(nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.UpdateRecurringPlanResponse{}
	err := suite.service.UpdateRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
}

func (suite *RecurringTestSuite) TestEnableRecurringPlanGetByIdError() {
	req := &billingpb.EnableRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(nil, errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.EnableRecurringPlanResponse{}
	err := suite.service.EnableRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanNotFound, res.Message)
}

func (suite *RecurringTestSuite) TestEnableRecurringPlanUpdateError() {
	req := &billingpb.EnableRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.PlanId,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
		Status: pkg.RecurringPlanStatusDisabled,
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(plan, nil)
	planRep.On("Update", mock.Anything, plan).Return(errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.EnableRecurringPlanResponse{}
	err := suite.service.EnableRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanUpdate, res.Message)
}

func (suite *RecurringTestSuite) TestEnableRecurringPlanOk() {
	req := &billingpb.EnableRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.PlanId,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
		Status: pkg.RecurringPlanStatusDisabled,
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(plan, nil)
	planRep.On("Update", mock.Anything, plan).Return(nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.EnableRecurringPlanResponse{}
	err := suite.service.EnableRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), pkg.RecurringPlanStatusActive, plan.Status)
}

func (suite *RecurringTestSuite) TestDisableRecurringPlanGetByIdError() {
	req := &billingpb.DisableRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(nil, errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.DisableRecurringPlanResponse{}
	err := suite.service.DisableRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanNotFound, res.Message)
}

func (suite *RecurringTestSuite) TestDisableRecurringPlanUpdateError() {
	req := &billingpb.DisableRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.PlanId,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
		Status: pkg.RecurringPlanStatusActive,
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(plan, nil)
	planRep.On("Update", mock.Anything, plan).Return(errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.DisableRecurringPlanResponse{}
	err := suite.service.DisableRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanUpdate, res.Message)
}

func (suite *RecurringTestSuite) TestDisableRecurringPlanOk() {
	req := &billingpb.DisableRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.PlanId,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
		Status: pkg.RecurringPlanStatusDisabled,
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(plan, nil)
	planRep.On("Update", mock.Anything, plan).Return(nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.DisableRecurringPlanResponse{}
	err := suite.service.DisableRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), pkg.RecurringPlanStatusDisabled, plan.Status)
}

func (suite *RecurringTestSuite) TestDeleteRecurringPlanGetByIdError() {
	req := &billingpb.DeleteRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(nil, errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.DeleteRecurringPlanResponse{}
	err := suite.service.DeleteRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanNotFound, res.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringPlanUpdateError() {
	req := &billingpb.DeleteRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.PlanId,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(plan, nil)
	planRep.On("Update", mock.Anything, plan).Return(errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.DeleteRecurringPlanResponse{}
	err := suite.service.DeleteRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanUpdate, res.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringPlanOk() {
	req := &billingpb.DeleteRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.PlanId,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(plan, nil)
	planRep.On("Update", mock.Anything, plan).Return(nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.DeleteRecurringPlanResponse{}
	err := suite.service.DeleteRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.NotEmpty(suite.T(), plan.DeletedAt)
}

func (suite *RecurringTestSuite) TestGetRecurringPlanGetByIdError() {
	req := &billingpb.GetRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(nil, errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.GetRecurringPlanResponse{}
	err := suite.service.GetRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), recurringErrorPlanNotFound, res.Message)
}

func (suite *RecurringTestSuite) TestGetRecurringPlanOk() {
	req := &billingpb.GetRecurringPlanRequest{
		PlanId:     primitive.NewObjectID().Hex(),
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.PlanId,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("GetById", mock.Anything, req.PlanId).Return(plan, nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.GetRecurringPlanResponse{}
	err := suite.service.GetRecurringPlan(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), plan, res.Item)
}

func (suite *RecurringTestSuite) TestGetRecurringPlansFindCountError() {
	req := &billingpb.GetRecurringPlansRequest{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		ExternalId: "ext",
		GroupId:    "group",
		Query:      "query",
		Offset:     0,
		Limit:      1,
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("FindCount", mock.Anything, req.MerchantId, req.ProjectId, req.ExternalId, req.GroupId, req.Query).
		Return(int64(0), errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.GetRecurringPlansResponse{}
	err := suite.service.GetRecurringPlans(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, res.Message)
}

func (suite *RecurringTestSuite) TestGetRecurringPlansFindError() {
	req := &billingpb.GetRecurringPlansRequest{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		ExternalId: "ext",
		GroupId:    "group",
		Query:      "query",
		Offset:     0,
		Limit:      1,
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("FindCount", mock.Anything, req.MerchantId, req.ProjectId, req.ExternalId, req.GroupId, req.Query).
		Return(int64(1), nil)
	planRep.On("Find", mock.Anything, req.MerchantId, req.ProjectId, req.ExternalId, req.GroupId, req.Query, req.Offset, req.Limit).
		Return(nil, errors.New("error"))
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.GetRecurringPlansResponse{}
	err := suite.service.GetRecurringPlans(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, res.Message)
}

func (suite *RecurringTestSuite) TestGetRecurringPlansOk() {
	req := &billingpb.GetRecurringPlansRequest{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		ExternalId: "ext",
		GroupId:    "group",
		Query:      "query",
		Offset:     0,
		Limit:      1,
	}
	plan := &billingpb.RecurringPlan{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  recurringpb.RecurringPeriodMinute,
			},
		},
	}

	merchantRep := &mocks.MerchantRepositoryInterface{}
	merchantRep.On("GetById", mock.Anything, req.MerchantId).Return(&billingpb.Merchant{Id: req.MerchantId}, nil)
	suite.service.merchantRepository = merchantRep

	projectRep := &mocks.ProjectRepositoryInterface{}
	projectRep.On("GetById", mock.Anything, req.ProjectId).Return(&billingpb.Project{MerchantId: req.MerchantId}, nil)
	suite.service.project = projectRep

	planRep := &mocks.RecurringPlanRepositoryInterface{}
	planRep.On("FindCount", mock.Anything, req.MerchantId, req.ProjectId, req.ExternalId, req.GroupId, req.Query).
		Return(int64(1), nil)
	planRep.On("Find", mock.Anything, req.MerchantId, req.ProjectId, req.ExternalId, req.GroupId, req.Query, req.Offset, req.Limit).
		Return([]*billingpb.RecurringPlan{plan}, nil)
	suite.service.recurringPlanRepository = planRep

	res := &billingpb.GetRecurringPlansResponse{}
	err := suite.service.GetRecurringPlans(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), int32(1), res.Count)
	assert.Len(suite.T(), res.List, 1)
}
