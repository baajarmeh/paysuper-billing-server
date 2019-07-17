package service

import (
	"context"
	"errors"
	rabbitmq "github.com/ProtocolONE/rabbitmq/pkg"
	"github.com/elliotchance/redismock"
	"github.com/globalsign/mgo/bson"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/mock"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	mongodb "github.com/paysuper/paysuper-database-mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
)

type UserProfileTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger
	cache   CacheInterface

	merchant          *billing.Merchant
	merchantAgreement *billing.Merchant
	merchant1         *billing.Merchant

	project *billing.Project

	pmBankCard *billing.PaymentMethod
	pmQiwi     *billing.PaymentMethod
}

func Test_UserProfile(t *testing.T) {
	suite.Run(t, new(UserProfileTestSuite))
}

func (suite *UserProfileTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	cfg.AccountingCurrency = "RUB"
	cfg.CardPayApiUrl = "https://sandbox.cardpay.com"

	db, err := mongodb.NewDatabase()

	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	err = InitTestCurrency(db, []interface{}{rub})

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
	}

	suite.log, err = zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	broker, err := rabbitmq.NewBroker(cfg.BrokerAddress)

	if err != nil {
		suite.FailNow("Creating RabbitMQ publisher failed", "%v", err)
	}

	redisdb := mock.NewTestRedis()
	suite.cache = NewCacheRedis(redisdb)
	suite.service = NewBillingService(db, cfg, nil, nil, nil, broker, mock.NewTestRedis(), suite.cache)

	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}
}

func (suite *UserProfileTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *UserProfileTestSuite) TestUserProfile_CreateOrUpdateUserProfile_NewProfile_Ok() {
	req := &grpc.UserProfile{
		UserId: bson.NewObjectId().Hex(),
		Email: &grpc.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &grpc.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &grpc.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &grpc.GetUserProfileResponse{}

	profile := suite.service.getOnboardingProfileByUser(req.UserId)
	assert.Nil(suite.T(), profile)

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &grpc.UserProfile{}, rsp.Item)

	profile = suite.service.getOnboardingProfileByUser(req.UserId)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &grpc.UserProfile{}, rsp.Item)

	assert.Equal(suite.T(), profile.UserId, rsp.Item.UserId)
	assert.Equal(suite.T(), profile.LastStep, rsp.Item.LastStep)
	assert.Equal(suite.T(), profile.Personal, rsp.Item.Personal)
	assert.Equal(suite.T(), profile.Help, rsp.Item.Help)
	assert.NotEmpty(suite.T(), rsp.Item.CentrifugoToken)
}

func (suite *UserProfileTestSuite) TestUserProfile_CreateOrUpdateOnboardingProfile_ExistProfile_Ok() {
	req := &grpc.UserProfile{
		UserId: bson.NewObjectId().Hex(),
		Email: &grpc.UserProfileEmail{
			Email: "test@unit.test",
		},
		LastStep: "step1",
	}
	rsp := &grpc.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &grpc.UserProfile{}, rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.CentrifugoToken)

	req1 := &grpc.UserProfile{
		UserId: req.UserId,
		Personal: &grpc.UserProfilePersonal{
			FirstName: "test",
			LastName:  "test",
			Position:  "unit",
		},
		Help: &grpc.UserProfileHelp{
			ProductPromotionAndDevelopment: true,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          true,
		},
		Company: &grpc.UserProfileCompany{
			CompanyName:       "company name",
			Website:           "http://127.0.0.1",
			AnnualIncome:      &grpc.RangeInt{From: 10, To: 100000},
			NumberOfEmployees: &grpc.RangeInt{From: 10, To: 50},
			KindOfActivity:    "test",
			Monetization: &grpc.UserProfileCompanyMonetization{
				PaidSubscription:  true,
				InGameAdvertising: true,
				InGamePurchases:   true,
				PremiumAccess:     true,
				Other:             true,
			},
			Platforms: &grpc.UserProfileCompanyPlatforms{
				PcMac:        true,
				GameConsole:  true,
				MobileDevice: true,
				WebBrowser:   true,
				Other:        true,
			},
		},
	}

	rsp1 := &grpc.GetUserProfileResponse{}
	err = suite.service.CreateOrUpdateUserProfile(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.NotEmpty(suite.T(), rsp1.Item.CentrifugoToken)

	assert.Equal(suite.T(), rsp.Item.UserId, rsp1.Item.UserId)
	assert.NotEqual(suite.T(), rsp.Item.Personal, rsp1.Item.Personal)
	assert.NotEqual(suite.T(), rsp.Item.Help, rsp1.Item.Help)
	assert.NotEqual(suite.T(), rsp.Item.Company, rsp1.Item.Company)

	profile := suite.service.getOnboardingProfileByUser(req.UserId)
	assert.NotNil(suite.T(), profile)

	assert.Equal(suite.T(), profile.UserId, rsp1.Item.UserId)
	assert.Equal(suite.T(), profile.LastStep, rsp1.Item.LastStep)
	assert.Equal(suite.T(), profile.Personal, rsp1.Item.Personal)
	assert.Equal(suite.T(), profile.Help, rsp1.Item.Help)
	assert.Equal(suite.T(), profile.Company, rsp1.Item.Company)
}

func (suite *UserProfileTestSuite) TestUserProfile_GetOnboardingProfile_Ok() {
	req := &grpc.UserProfile{
		UserId: bson.NewObjectId().Hex(),
		Email: &grpc.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &grpc.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &grpc.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &grpc.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.IsType(suite.T(), &grpc.UserProfile{}, rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.CentrifugoToken)

	req1 := &grpc.GetUserProfileRequest{UserId: req.UserId}
	rsp1 := &grpc.GetUserProfileResponse{}
	err = suite.service.GetUserProfile(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.NotEmpty(suite.T(), rsp1.Item.CentrifugoToken)

	assert.Equal(suite.T(), rsp.Item.Id, rsp1.Item.Id)
	assert.Equal(suite.T(), rsp.Item.UserId, rsp1.Item.UserId)
	assert.Equal(suite.T(), rsp.Item.Personal, rsp1.Item.Personal)
	assert.Equal(suite.T(), rsp.Item.Help, rsp1.Item.Help)
	assert.Equal(suite.T(), rsp.Item.Company, rsp1.Item.Company)
	assert.Equal(suite.T(), rsp.Item.LastStep, rsp1.Item.LastStep)
}

func (suite *UserProfileTestSuite) TestUserProfile_GetOnboardingProfile_NotFound_Error() {
	req := &grpc.GetUserProfileRequest{UserId: bson.NewObjectId().Hex()}
	rsp := &grpc.GetUserProfileResponse{}
	err := suite.service.GetUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), userProfileErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *UserProfileTestSuite) TestUserProfile_SendConfirmEmailToUser_Ok() {
	req := &grpc.UserProfile{
		UserId: bson.NewObjectId().Hex(),
		Email: &grpc.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &grpc.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &grpc.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &grpc.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	req1 := &grpc.SendConfirmEmailToUserRequest{
		UserId: req.UserId,
		Host:   "http://localhost",
	}
	rsp1 := &grpc.GetUserProfileResponse{}
	err = suite.service.SendConfirmEmailToUser(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotEmpty(suite.T(), rsp1.Item.Email.ConfirmationUrl)
	assert.Regexp(suite.T(), req1.Host, rsp1.Item.Email.ConfirmationUrl)
}

func (suite *UserProfileTestSuite) TestUserProfile_SendConfirmEmailToUser_UserNotFound_Error() {
	req := &grpc.SendConfirmEmailToUserRequest{
		UserId: bson.NewObjectId().Hex(),
		Host:   "http://localhost",
	}
	rsp := &grpc.GetUserProfileResponse{}
	err := suite.service.SendConfirmEmailToUser(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), userProfileErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *UserProfileTestSuite) TestUserProfile_SendConfirmEmailToUser_EmailAlreadyVerified_Error() {
	req := &grpc.UserProfile{
		UserId: bson.NewObjectId().Hex(),
		Email: &grpc.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &grpc.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &grpc.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &grpc.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	rsp.Item.Email.Confirmed = true
	rsp.Item.Email.ConfirmedAt = ptypes.TimestampNow()

	err = suite.service.db.Collection(collectionUserProfile).UpdateId(bson.ObjectIdHex(rsp.Item.Id), rsp.Item)
	assert.NoError(suite.T(), err)

	req1 := &grpc.SendConfirmEmailToUserRequest{
		UserId: req.UserId,
		Host:   "http://localhost",
	}
	rsp1 := &grpc.GetUserProfileResponse{}
	err = suite.service.SendConfirmEmailToUser(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), userProfileEmailConfirmed, rsp1.Message)
}

func (suite *UserProfileTestSuite) TestUserProfile_SendConfirmEmailToUser_SaveTokenError() {
	req := &grpc.UserProfile{
		UserId: bson.NewObjectId().Hex(),
		Email: &grpc.UserProfileEmail{
			Email: "test@unit.test",
		},
		Personal: &grpc.UserProfilePersonal{
			FirstName: "Unit test",
			LastName:  "Unit Test",
			Position:  "test",
		},
		Help: &grpc.UserProfileHelp{
			ProductPromotionAndDevelopment: false,
			ReleasedGamePromotion:          true,
			InternationalSales:             true,
			Other:                          false,
		},
		LastStep: "step2",
	}
	rsp := &grpc.GetUserProfileResponse{}

	err := suite.service.CreateOrUpdateUserProfile(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	redisCl, ok := suite.service.redis.(*redismock.ClientMock)
	assert.True(suite.T(), ok)

	redisCl.On("Set").
		Return(redis.NewStatusResult("", errors.New("server not available")))

	req1 := &grpc.SendConfirmEmailToUserRequest{
		UserId: req.UserId,
		Host:   "http://localhost",
	}
	rsp1 := &grpc.GetUserProfileResponse{}
	err = suite.service.SendConfirmEmailToUser(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusSystemError, rsp1.Status)
	assert.Equal(suite.T(), userProfileErrorUnknown, rsp1.Message)
}
