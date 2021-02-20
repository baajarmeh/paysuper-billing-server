package service

import (
	"context"
	"errors"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
	"time"
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_Ok() {
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_IncorrectCookie_Error() {
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_DontHaveCustomerId_Error() {
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_RealCustomer_Ok() {
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_RealCustomerNotFound_Error() {
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_RecurringServiceSystem_Error() {
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_RecurringServiceResult_Error() {
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

func (suite *RecurringTestSuite) TestDeleteSavedCard_RecurringServiceResultSystemError_Error() {
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodDay,
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
				Type:  billingpb.RecurringPeriodWeek,
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
				Type:  billingpb.RecurringPeriodMonth,
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
				Type:  billingpb.RecurringPeriodYear,
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
				Type:  billingpb.RecurringPeriodDay,
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
				Type:  billingpb.RecurringPeriodDay,
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
				Type:  billingpb.RecurringPeriodDay,
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
				Type:  billingpb.RecurringPeriodDay,
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
				Type:  billingpb.RecurringPeriodDay,
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
				Type:  billingpb.RecurringPeriodDay,
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
			Type:  billingpb.RecurringPeriodDay,
		},
		Trial: &billingpb.RecurringPlanPeriod{
			Value: 1,
			Type:  billingpb.RecurringPeriodDay,
		},
		GracePeriod: &billingpb.RecurringPlanPeriod{
			Value: 1,
			Type:  billingpb.RecurringPeriodDay,
		},
	}
	plan := &billingpb.RecurringPlan{
		Id:         req.Id,
		MerchantId: req.MerchantId,
		ProjectId:  req.ProjectId,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 2,
				Type:  billingpb.RecurringPeriodMinute,
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
			Type:  billingpb.RecurringPeriodWeek,
		},
		Trial: &billingpb.RecurringPlanPeriod{
			Value: 4,
			Type:  billingpb.RecurringPeriodMonth,
		},
		GracePeriod: &billingpb.RecurringPlanPeriod{
			Value: 5,
			Type:  billingpb.RecurringPeriodYear,
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodMinute,
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
				Type:  billingpb.RecurringPeriodMinute,
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

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_WithoutCookie_Ok() {
	var (
		psId      = "payment_system_id"
		psHandler = "payment_system_handler"
	)

	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetBySubscriptionId", mock.Anything, mock.Anything).Return([]*billingpb.Order{order}, nil)
	suite.service.orderRepository = orderRepository

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, subscription.Id).Return(subscription, nil)
	subscriptionRepository.On("Update", mock.Anything, mock.MatchedBy(func(input *billingpb.RecurringSubscription) bool {
		return input.Id == subscription.Id && input.Status == billingpb.RecurringSubscriptionStatusCanceled
	})).Return(nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

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
		Id:         subscription.Id,
		MerchantId: subscription.Plan.MerchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_WithCookie_Ok() {
	var (
		psId      = "payment_system_id"
		psHandler = "payment_system_handler"
	)

	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	browserCustomer := &BrowserCookieCustomer{
		CustomerId:     subscription.Customer.Id,
		Ip:             "127.0.0.1",
		AcceptLanguage: "fr-CA",
		UserAgent:      "windows",
		SessionCount:   0,
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetBySubscriptionId", mock.Anything, mock.Anything).Return([]*billingpb.Order{order}, nil)
	suite.service.orderRepository = orderRepository

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, subscription.Id).Return(subscription, nil)
	subscriptionRepository.On("Update", mock.Anything, mock.MatchedBy(func(input *billingpb.RecurringSubscription) bool {
		return input.Id == subscription.Id && input.Status == billingpb.RecurringSubscriptionStatusCanceled
	})).Return(nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

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
		Id:     subscription.Id,
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_BadCookie_Error() {
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:     "id",
		Cookie: "cookie",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringCustomerNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_NoCustomerOnCookie_Error() {
	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, mock.Anything).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

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

	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:     "id",
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_SubscriptionNotFound_Error() {
	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, mock.Anything).Return(nil, errors.New("error"))
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id: "recurring_id",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorRecurringSubscriptionNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_AccessDenyByEmptyIdentifiers_Error() {
	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, mock.Anything).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id: "recurring_id",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_AccessDenyByMerchant_Error() {
	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, mock.Anything).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         subscription.Id,
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

	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, mock.Anything).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:     subscription.Id,
		Cookie: cookie,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, rsp.Status)
	assert.Equal(suite.T(), recurringErrorAccessDeny, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_OrderNotFound_Error() {
	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, mock.Anything).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetBySubscriptionId", mock.Anything, mock.Anything).Return(nil, errors.New("notfound"))
	suite.service.orderRepository = orderRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         subscription.Id,
		MerchantId: subscription.Plan.MerchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorNotFound, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_PaymentSystemNotFound_Error() {
	var (
		psId = "payment_system_id"
	)

	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetBySubscriptionId", mock.Anything, mock.Anything).Return([]*billingpb.Order{order}, nil)
	suite.service.orderRepository = orderRepository

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, subscription.Id).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	psRepository := &mocks.PaymentSystemRepositoryInterface{}
	psRepository.On("GetById", mock.Anything, psId).Return(nil, errors.New("notfound"))
	suite.service.paymentSystemRepository = psRepository

	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.DeleteRecurringSubscription(context.Background(), &billingpb.DeleteRecurringSubscriptionRequest{
		Id:         subscription.Id,
		MerchantId: subscription.Plan.MerchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorPaymentSystemInactive, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_PaymentSystemGatewayNotFound_Error() {
	var (
		psId      = "payment_system_id"
		psHandler = "payment_system_handler"
	)

	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetBySubscriptionId", mock.Anything, mock.Anything).Return([]*billingpb.Order{order}, nil)
	suite.service.orderRepository = orderRepository

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, subscription.Id).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

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
		Id:         subscription.Id,
		MerchantId: subscription.Plan.MerchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), orderErrorPaymentSystemInactive, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_DeleteSubscriptionOnPaymentSystem_Error() {
	var (
		psId      = "payment_system_id"
		psHandler = "payment_system_handler"
	)

	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetBySubscriptionId", mock.Anything, mock.Anything).Return([]*billingpb.Order{order}, nil)
	suite.service.orderRepository = orderRepository

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, subscription.Id).Return(subscription, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

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
		Id:         subscription.Id,
		MerchantId: subscription.Plan.MerchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorDeleteSubscription, rsp.Message)
}

func (suite *RecurringTestSuite) TestDeleteRecurringSubscription_DeleteFromRepository_Error() {
	var (
		psId      = "payment_system_id"
		psHandler = "payment_system_handler"
	)

	subscription := &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			MerchantId: primitive.NewObjectID().Hex(),
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: primitive.NewObjectID().Hex(),
		},
	}
	order := &billingpb.Order{
		PaymentMethod: &billingpb.PaymentMethodOrder{
			PaymentSystemId: psId,
		},
	}

	orderRepository := &mocks.OrderRepositoryInterface{}
	orderRepository.On("GetBySubscriptionId", mock.Anything, mock.Anything).Return([]*billingpb.Order{order}, nil)
	suite.service.orderRepository = orderRepository

	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("GetById", mock.Anything, subscription.Id).Return(subscription, nil)
	subscriptionRepository.On("Update", mock.Anything, mock.Anything).Return(errors.New("error"))
	suite.service.recurringSubscriptionRepository = subscriptionRepository

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
		Id:         subscription.Id,
		MerchantId: subscription.Plan.MerchantId,
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorDeleteSubscription, rsp.Message)
}

func (suite *RecurringTestSuite) TestFindExpiredSubscriptions_InvalidExpireAt() {
	rsp := &billingpb.FindExpiredSubscriptionsResponse{}
	err := suite.service.FindExpiredSubscriptions(context.Background(), &billingpb.FindExpiredSubscriptionsRequest{
		ExpireAt: "time",
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), recurringErrorInvalidExpireDate, rsp.Message)
}

func (suite *RecurringTestSuite) TestFindExpiredSubscriptions_RepositoryError() {
	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("FindExpired", mock.Anything, mock.Anything).Return(nil, errors.New("error"))
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	rsp := &billingpb.FindExpiredSubscriptionsResponse{}
	err := suite.service.FindExpiredSubscriptions(context.Background(), &billingpb.FindExpiredSubscriptionsRequest{
		ExpireAt: time.Now().UTC().Format("2006-01-02"),
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), recurringErrorUnknown, rsp.Message)
}

func (suite *RecurringTestSuite) TestFindExpiredSubscriptions_Ok() {
	subscriptionRepository := &mocks.RecurringSubscriptionRepositoryInterface{}
	subscriptionRepository.On("FindExpired", mock.Anything, mock.Anything).Return([]*billingpb.RecurringSubscription{}, nil)
	suite.service.recurringSubscriptionRepository = subscriptionRepository

	rsp := &billingpb.FindExpiredSubscriptionsResponse{}
	err := suite.service.FindExpiredSubscriptions(context.Background(), &billingpb.FindExpiredSubscriptionsRequest{
		ExpireAt: time.Now().UTC().Format("2006-01-02"),
	}, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
}
