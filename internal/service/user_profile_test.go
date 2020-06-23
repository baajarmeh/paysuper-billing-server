package service

import (
	"context"
	"errors"
	"github.com/elliotchance/redismock"
	"github.com/go-redis/redis"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/casbinpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"net/url"
	"testing"
)

type UserProfileTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger
	cache   database.CacheInterface

	merchant          *billingpb.Merchant
	merchantAgreement *billingpb.Merchant
	merchant1         *billingpb.Merchant

	project *billingpb.Project

	pmBankCard *billingpb.PaymentMethod
	pmQiwi     *billingpb.PaymentMethod

	logObserver *zap.Logger
	zapRecorder *observer.ObservedLogs
}

func Test_UserProfile(t *testing.T) {
	suite.Run(t, new(UserProfileTestSuite))
}

func (suite *UserProfileTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	cfg.CardPayApiUrl = "https://sandbox.cardpay.com"

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
		mocks.NewGeoIpServiceTestOk(),
		mocks.NewRepositoryServiceOk(),
		mocks.NewTaxServiceOkMock(),
		mocks.NewBrokerMockOk(),
		mocks.NewTestRedis(),
		suite.cache,
		mocks.NewCurrencyServiceMockOk(),
		mocks.NewDocumentSignerMockOk(),
		&reportingMocks.ReporterService{},
		mocks.NewFormatterOK(),
		mocks.NewBrokerMockOk(),
		&casbinMocks.CasbinService{},
		nil,
		mocks.NewBrokerMockOk(),
	)

	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	country := &billingpb.Country{
		IsoCodeA2:       "RU",
		Region:          "Russia",
		Currency:        "RUB",
		PaymentsAllowed: true,
		ChangeAllowed:   true,
		VatEnabled:      true,
		PriceGroupId:    "",
		VatCurrency:     "RUB",
	}

	if err := suite.service.country.Insert(context.TODO(), country); err != nil {
		suite.FailNow("Insert country test data failed", "%v", err)
	}

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)
}

func (suite *UserProfileTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *UserProfileTestSuite) TestUserProfile_CreateOrUpdateUserProfile_NewProfile_Ok() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	profile, err := suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), profile)

	err = suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &billingpb.UserProfile{}, rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.Id)
	assert.NotEmpty(suite.T(), rsp.Item.CreatedAt)
	assert.NotEmpty(suite.T(), rsp.Item.UpdatedAt)

	profile, err = suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), profile)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &billingpb.UserProfile{}, rsp.Item)

	assert.Equal(suite.T(), profile.UserId, rsp.Item.UserId)
	assert.Equal(suite.T(), profile.LastStep, rsp.Item.LastStep)
	assert.Equal(suite.T(), profile.Personal.LastName, rsp.Item.Personal.LastName)
	assert.Equal(suite.T(), profile.Personal.FirstName, rsp.Item.Personal.FirstName)
	assert.Equal(suite.T(), profile.Personal.Position, rsp.Item.Personal.Position)
	assert.Equal(suite.T(), profile.Help.Other, rsp.Item.Help.Other)
	assert.Equal(suite.T(), profile.Help.InternationalSales, rsp.Item.Help.InternationalSales)
	assert.Equal(suite.T(), profile.Help.ReleasedGamePromotion, rsp.Item.Help.ReleasedGamePromotion)
	assert.Equal(suite.T(), profile.Help.ProductPromotionAndDevelopment, rsp.Item.Help.ProductPromotionAndDevelopment)
	assert.NotEmpty(suite.T(), rsp.Item.CentrifugoToken)

	b, ok := suite.service.postmarkBroker.(*mocks.BrokerMockOk)
	assert.True(suite.T(), ok)
	assert.False(suite.T(), b.IsSent)
}

func (suite *UserProfileTestSuite) TestUserProfile_CreateOrUpdateUserProfile_ChangeProfileWithSendConfirmEmail_Ok() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	profile, err := suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), profile)

	err = suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &billingpb.UserProfile{}, rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.Id)
	assert.NotEmpty(suite.T(), rsp.Item.CreatedAt)
	assert.NotEmpty(suite.T(), rsp.Item.UpdatedAt)

	req = &billingpb.UserProfile{
		UserId: req.UserId,
		Email:  req.Email,
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	err = suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	profile, err = suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), profile)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &billingpb.UserProfile{}, rsp.Item)

	assert.Equal(suite.T(), profile.UserId, rsp.Item.UserId)
	assert.Equal(suite.T(), profile.LastStep, rsp.Item.LastStep)
	assert.Equal(suite.T(), profile.Personal.LastName, rsp.Item.Personal.LastName)
	assert.Equal(suite.T(), profile.Personal.FirstName, rsp.Item.Personal.FirstName)
	assert.Equal(suite.T(), profile.Personal.Position, rsp.Item.Personal.Position)
	assert.Equal(suite.T(), profile.Help.Other, rsp.Item.Help.Other)
	assert.Equal(suite.T(), profile.Help.InternationalSales, rsp.Item.Help.InternationalSales)
	assert.Equal(suite.T(), profile.Help.ReleasedGamePromotion, rsp.Item.Help.ReleasedGamePromotion)
	assert.Equal(suite.T(), profile.Help.ProductPromotionAndDevelopment, rsp.Item.Help.ProductPromotionAndDevelopment)
	assert.NotEmpty(suite.T(), rsp.Item.CentrifugoToken)

	b, ok := suite.service.postmarkBroker.(*mocks.BrokerMockOk)
	assert.True(suite.T(), ok)
	assert.True(suite.T(), b.IsSent)
}

func (suite *UserProfileTestSuite) TestUserProfile_CreateOrUpdateOnboardingProfile_ExistProfile_Ok() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &billingpb.UserProfile{}, rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.CentrifugoToken)

	b, ok := suite.service.postmarkBroker.(*mocks.BrokerMockOk)
	assert.True(suite.T(), ok)
	assert.True(suite.T(), b.IsSent)

	b.IsSent = false

	req1 := &billingpb.UserProfile{
		UserId: req.UserId,
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "test",
			LastName:  "test",
			Position:  "unit",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: true,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          true,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "company name",
			Website:           "http://127.0.0.1",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100000},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 50},
			KindOfActivity:    "test",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription:  true,
				InGameAdvertising: true,
				InGamePurchases:   true,
				PremiumAccess:     true,
				Other:             true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				PcMac:        true,
				GameConsole:  true,
				MobileDevice: true,
				WebBrowser:   true,
				Other:        true,
			},
		},
	}

	rsp1 := &billingpb.GetUserProfileResponse{}
	err = suite.service.CreateOrUpdateUserProfile(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.NotEmpty(suite.T(), rsp1.Item.CentrifugoToken)

	assert.Equal(suite.T(), rsp.Item.UserId, rsp1.Item.UserId)
	assert.NotEqual(suite.T(), rsp.Item.Personal, rsp1.Item.Personal)
	assert.NotEqual(suite.T(), rsp.Item.Help, rsp1.Item.Help)
	assert.NotEqual(suite.T(), rsp.Item.Company, rsp1.Item.Company)

	profile, err := suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), profile)

	assert.Equal(suite.T(), profile.UserId, rsp1.Item.UserId)
	assert.Equal(suite.T(), profile.LastStep, rsp1.Item.LastStep)
	assert.Equal(suite.T(), profile.Personal, rsp1.Item.Personal)
	assert.Equal(suite.T(), profile.Help, rsp1.Item.Help)
	assert.Equal(suite.T(), profile.Company, rsp1.Item.Company)

	assert.False(suite.T(), b.IsSent)
}

func (suite *UserProfileTestSuite) TestUserProfile_CreateOrUpdateUserProfile_NewProfile_SetUserEmailConfirmationToken_Error() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	profile, err := suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), profile)

	redisCl, ok := suite.service.redis.(*redismock.ClientMock)
	assert.True(suite.T(), ok)

	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)

	redisCl.On("Set").
		Return(redis.NewStatusResult("", errors.New("server not available")))

	err = suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), userProfileErrorUnknown, rsp.Message)

	messages := recorded.All()
	assert.Contains(suite.T(), messages[1].Message, "Save confirm email token to Redis failed")
}

func (suite *UserProfileTestSuite) TestUserProfile_CreateOrUpdateUserProfile_NewProfile_SendUserEmailConfirmationToken_Error() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	profile, err := suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), profile)

	suite.service.postmarkBroker = mocks.NewBrokerMockError()

	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)

	err = suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp.Status)
	assert.Equal(suite.T(), userProfileErrorUnknown, rsp.Message)

	messages := recorded.All()
	assert.Contains(suite.T(), messages[1].Message, "Publication message to user email confirmation to queue failed")
}

func (suite *UserProfileTestSuite) TestUserProfile_GetOnboardingProfile_Ok() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &billingpb.UserProfile{}, rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.CentrifugoToken)

	req1 := &billingpb.GetUserProfileRequest{UserId: req.UserId}
	rsp1 := &billingpb.GetUserProfileResponse{}
	err = suite.service.GetUserProfile(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.NotEmpty(suite.T(), rsp1.Item.CentrifugoToken)

	assert.Equal(suite.T(), rsp.Item.Id, rsp1.Item.Id)
	assert.Equal(suite.T(), rsp.Item.UserId, rsp1.Item.UserId)
	assert.Equal(suite.T(), rsp.Item.Personal.LastName, rsp1.Item.Personal.LastName)
	assert.Equal(suite.T(), rsp.Item.Personal.FirstName, rsp1.Item.Personal.FirstName)
	assert.Equal(suite.T(), rsp.Item.Personal.Position, rsp1.Item.Personal.Position)
	assert.Equal(suite.T(), rsp.Item.Help.Other, rsp1.Item.Help.Other)
	assert.Equal(suite.T(), rsp.Item.Help.InternationalSales, rsp1.Item.Help.InternationalSales)
	assert.Equal(suite.T(), rsp.Item.Help.ReleasedGamePromotion, rsp1.Item.Help.ReleasedGamePromotion)
	assert.Equal(suite.T(), rsp.Item.Help.ProductPromotionAndDevelopment, rsp1.Item.Help.ProductPromotionAndDevelopment)
	assert.Equal(suite.T(), rsp.Item.Company, rsp1.Item.Company)
	assert.Equal(suite.T(), rsp.Item.LastStep, rsp1.Item.LastStep)
}

func (suite *UserProfileTestSuite) TestUserProfile_GetOnboardingProfile_NotFound_Error() {
	req := &billingpb.GetUserProfileRequest{UserId: primitive.NewObjectID().Hex()}
	rsp := &billingpb.GetUserProfileResponse{}
	err := suite.service.GetUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), userProfileErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *UserProfileTestSuite) TestUserProfile_ConfirmUserEmail_Ok() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	u, err := url.ParseRequestURI(rsp.Item.Email.ConfirmationUrl)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), u)
	assert.NotEmpty(suite.T(), u.RawQuery)

	p, err := url.ParseQuery(u.RawQuery)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), p, 1)
	assert.Contains(suite.T(), p, "token")

	zap.ReplaceGlobals(suite.logObserver)
	suite.service.centrifugoDashboard = newCentrifugo(suite.service.cfg.CentrifugoDashboard, mocks.NewClientStatusOk())

	req2 := &billingpb.ConfirmUserEmailRequest{Token: p["token"][0]}
	rsp2 := &billingpb.ConfirmUserEmailResponse{}
	err = suite.service.ConfirmUserEmail(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	messages := suite.zapRecorder.All()
	assert.Regexp(suite.T(), "dashboard", messages[0].Message)

	profile, err := suite.service.userProfileRepository.GetByUserId(context.TODO(), req.UserId)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), profile)
	assert.True(suite.T(), profile.Email.Confirmed)
	assert.NotNil(suite.T(), profile.Email.ConfirmedAt)
}

func (suite *UserProfileTestSuite) TestUserProfile_ConfirmUserEmail_TokenNotFound_Error() {
	req := &billingpb.ConfirmUserEmailRequest{Token: primitive.NewObjectID().Hex()}
	rsp := &billingpb.ConfirmUserEmailResponse{}
	err := suite.service.ConfirmUserEmail(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), userProfileEmailConfirmationTokenNotFound, rsp.Message)
}

func (suite *UserProfileTestSuite) TestUserProfile_ConfirmUserEmail_UserNotFound_Error() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	u, err := url.ParseRequestURI(rsp.Item.Email.ConfirmationUrl)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), u)
	assert.NotEmpty(suite.T(), u.RawQuery)

	p, err := url.ParseQuery(u.RawQuery)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), p, 1)
	assert.Contains(suite.T(), p, "token")

	token := p["token"][0]
	err = suite.service.redis.Set(
		suite.service.getConfirmEmailStorageKey(token),
		primitive.NewObjectID().Hex(),
		suite.service.cfg.GetEmailConfirmTokenLifetime(),
	).Err()
	assert.NoError(suite.T(), err)

	req2 := &billingpb.ConfirmUserEmailRequest{Token: token}
	rsp2 := &billingpb.ConfirmUserEmailResponse{}
	err = suite.service.ConfirmUserEmail(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp2.Status)
	assert.Equal(suite.T(), userProfileErrorUnknown, rsp2.Message)
}

func (suite *UserProfileTestSuite) TestUserProfile_ConfirmUserEmail_EmailAlreadyConfirmed_Error() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	u, err := url.ParseRequestURI(rsp.Item.Email.ConfirmationUrl)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), u)
	assert.NotEmpty(suite.T(), u.RawQuery)

	p, err := url.ParseQuery(u.RawQuery)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), p, 1)
	assert.Contains(suite.T(), p, "token")

	ci := &mocks.CentrifugoInterface{}
	ci.On("Publish", mock2.Anything, mock2.Anything, mock2.Anything).Return(nil)
	suite.service.centrifugoDashboard = ci

	req2 := &billingpb.ConfirmUserEmailRequest{Token: p["token"][0]}
	rsp2 := &billingpb.ConfirmUserEmailResponse{}
	err = suite.service.ConfirmUserEmail(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	err = suite.service.ConfirmUserEmail(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
}

func (suite *UserProfileTestSuite) TestUserProfile_ConfirmUserEmail_EmailConfirmedSuccessfully_Error() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		Company: &billingpb.UserProfileCompany{
			CompanyName:       "Unit test",
			Website:           "http://localhost",
			AnnualIncome:      &billingpb.RangeInt{From: 10, To: 100},
			NumberOfEmployees: &billingpb.RangeInt{From: 10, To: 100},
			KindOfActivity:    "develop_and_publish_your_games",
			Monetization: &billingpb.UserProfileCompanyMonetization{
				PaidSubscription: true,
			},
			Platforms: &billingpb.UserProfileCompanyPlatforms{
				WebBrowser: true,
			},
		},
		LastStep: "step3",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	u, err := url.ParseRequestURI(rsp.Item.Email.ConfirmationUrl)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), u)
	assert.NotEmpty(suite.T(), u.RawQuery)

	p, err := url.ParseQuery(u.RawQuery)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), p, 1)
	assert.Contains(suite.T(), p, "token")

	req2 := &billingpb.ConfirmUserEmailRequest{Token: p["token"][0]}
	rsp2 := &billingpb.ConfirmUserEmailResponse{}
	err = suite.service.ConfirmUserEmail(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp2.Status)
	assert.Equal(suite.T(), userProfileErrorUnknown, rsp2.Message)
}

func (suite *UserProfileTestSuite) TestUserProfile_CreatePageReview_Ok() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &billingpb.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	req1 := &billingpb.CreatePageReviewRequest{
		UserId: req.UserId,
		Review: "review 1",
		Url:    "primary_onboarding",
	}
	rsp1 := &billingpb.CheckProjectRequestSignatureResponse{}
	err = suite.service.CreatePageReview(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)

	req1.Review = "review 2"
	err = suite.service.CreatePageReview(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)

	req1.Review = "review 3"
	err = suite.service.CreatePageReview(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)

	reviews, err := suite.service.feedbackRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), reviews, 3)

	for _, v := range reviews {
		assert.NotEmpty(suite.T(), v.UserId)
		assert.NotEmpty(suite.T(), v.Review)
		assert.NotEmpty(suite.T(), v.Url)
	}
}

func (suite *UserProfileTestSuite) TestUserProfile_GetUserProfile_ByProfileId_Ok() {
	req := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &billingpb.GetUserProfileResponse{}
	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	req1 := &billingpb.GetUserProfileRequest{ProfileId: rsp.Item.Id}
	rsp1 := &billingpb.GetUserProfileResponse{}
	err = suite.service.GetUserProfile(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.Equal(suite.T(), rsp.Item.Id, rsp1.Item.Id)
	assert.Equal(suite.T(), rsp.Item.UserId, rsp1.Item.UserId)
}

func (suite *UserProfileTestSuite) TestUserProfile_GetCommonUserProfile_HasProjectsTrue() {
	ctx := context.TODO()
	userProfile := &billingpb.UserProfile{
		UserId: primitive.NewObjectID().Hex(),
		Email: &billingpb.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &billingpb.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &billingpb.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	err := suite.service.userProfileRepository.Add(ctx, userProfile)
	assert.NoError(suite.T(), err)

	merchant := &billingpb.Merchant{
		Id:      primitive.NewObjectID().Hex(),
		Company: &billingpb.MerchantCompanyInfo{Name: "name"},
		Banking: &billingpb.MerchantBanking{Currency: "currency"},
	}
	err = suite.service.merchantRepository.Insert(ctx, merchant)
	assert.NoError(suite.T(), err)

	role := &billingpb.UserRole{
		Id:         primitive.NewObjectID().Hex(),
		UserId:     userProfile.UserId,
		MerchantId: merchant.Id,
		Role:       billingpb.RoleMerchantOwner,
	}
	err = suite.service.userRoleRepository.AddMerchantUser(ctx, role)
	assert.NoError(suite.T(), err)

	project := &billingpb.Project{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: merchant.Id,
	}
	err = suite.service.project.Insert(ctx, project)
	assert.NoError(suite.T(), err)

	casbin := &casbinMocks.CasbinService{}
	casbin.
		On("GetImplicitPermissionsForUser", mock2.Anything, mock2.Anything).
		Return(&casbinpb.Array2DReply{D2: nil}, nil)
	suite.service.casbinService = casbin

	req := &billingpb.CommonUserProfileRequest{UserId: userProfile.UserId, MerchantId: merchant.Id}
	rsp := &billingpb.CommonUserProfileResponse{}
	err = suite.service.GetCommonUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Profile.Merchant)
	assert.Equal(suite.T(), merchant.Id, rsp.Profile.Merchant.Id)
	assert.True(suite.T(), rsp.Profile.Merchant.HasProjects)
}
