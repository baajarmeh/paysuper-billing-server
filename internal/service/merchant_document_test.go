package service

import (
	"context"
	"github.com/go-redis/redis"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/internal/payment_system"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"net/http"
	"testing"
)

type MerchantDocumentTestSuite struct {
	suite.Suite
	service    *Service
	log        *zap.Logger
	cache      database.CacheInterface
	httpClient *http.Client

	logObserver *zap.Logger
	zapRecorder *observer.ObservedLogs

	merchant  *billingpb.Merchant
	merchant2 *billingpb.Merchant
}

func Test_MerchantDocument(t *testing.T) {
	suite.Run(t, new(MerchantDocumentTestSuite))
}

func (suite *MerchantDocumentTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}
	cfg.CardPayApiUrl = "https://sandbox.cardpay.com"

	m, err := migrate.New(
		"file://../../migrations/tests",
		cfg.MongoDsn)
	assert.NoError(suite.T(), err, "Migrate init failed")

	err = m.Up()
	if err != nil && err.Error() != "no change" {
		suite.FailNow("Migrations failed", "%v", err)
	}

	db, err := mongodb.NewDatabase()
	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	suite.log, err = zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	broker, err := rabbitmq.NewBroker(cfg.BrokerAddress)

	if err != nil {
		suite.FailNow("Creating RabbitMQ publisher failed", "%v", err)
	}

	redisClient := database.NewRedis(
		&redis.Options{
			Addr:     cfg.RedisHost,
			Password: cfg.RedisPassword,
		},
	)

	redisdb := mocks.NewTestRedis()
	suite.httpClient = payment_system.NewClientStatusOk()
	suite.cache, err = database.NewCacheRedis(redisdb, "cache")
	suite.service = NewBillingService(
		db,
		cfg,
		mocks.NewGeoIpServiceTestOk(),
		mocks.NewRepositoryServiceOk(),
		mocks.NewTaxServiceOkMock(),
		broker,
		redisClient,
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

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)

	operatingCompany := HelperOperatingCompany(suite.Suite, suite.service)

	suite.merchant = HelperCreateMerchant(suite.Suite, suite.service, "RUB", "RU", nil, 13000, operatingCompany.Id)
	suite.merchant2 = HelperCreateMerchant(suite.Suite, suite.service, "", "RU", nil, 0, operatingCompany.Id)
}

func (suite *MerchantDocumentTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_AddMerchantDocument_Ok() {
	suite.merchant.Status = billingpb.MerchantStatusPending
	err := suite.service.merchantRepository.Update(context.Background(), suite.merchant)
	assert.NoError(suite.T(), err)

	res := &billingpb.AddMerchantDocumentResponse{}
	err = suite.service.AddMerchantDocument(context.Background(), &billingpb.MerchantDocument{
		MerchantId: suite.merchant.Id,
		UserId:     primitive.NewObjectID().Hex(),
	}, res)

	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), res.Item)
	assert.NotEmpty(suite.T(), res.Item.Id)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)

	b, ok := suite.service.postmarkBroker.(*mocks.BrokerMockOk)
	assert.True(suite.T(), ok)
	assert.True(suite.T(), b.IsSent)

	merchant, err := suite.service.merchantRepository.GetById(context.Background(), suite.merchant.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), merchant)
	assert.Equal(suite.T(), billingpb.MerchantStatusKycStarted, merchant.Status)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_AddMerchantDocument_Error_MerchantNotFound() {
	res := &billingpb.AddMerchantDocumentResponse{}
	err := suite.service.AddMerchantDocument(context.Background(), &billingpb.MerchantDocument{
		MerchantId: primitive.NewObjectID().Hex(),
	}, res)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), errorMerchantNotFound, res.Message)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_AddMerchantDocument_Error_MerchantStatus() {
	res := &billingpb.AddMerchantDocumentResponse{}
	err := suite.service.AddMerchantDocument(context.Background(), &billingpb.MerchantDocument{
		MerchantId: suite.merchant.Id,
	}, res)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), errorMerchantDocumentIncorrectStatus, res.Message)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetMerchantDocument_Ok() {
	suite.merchant.Status = billingpb.MerchantStatusPending
	err := suite.service.merchantRepository.Update(context.Background(), suite.merchant)
	assert.NoError(suite.T(), err)

	res := &billingpb.AddMerchantDocumentResponse{}
	err = suite.service.AddMerchantDocument(context.Background(), &billingpb.MerchantDocument{
		MerchantId: suite.merchant.Id,
		UserId:     primitive.NewObjectID().Hex(),
	}, res)
	assert.NoError(suite.T(), err)

	res2 := &billingpb.GetMerchantDocumentResponse{}
	err = suite.service.GetMerchantDocument(context.Background(), &billingpb.GetMerchantDocumentRequest{
		Id:         res.Item.Id,
		MerchantId: res.Item.MerchantId,
	}, res2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.NotEmpty(suite.T(), res2.Item)
	assert.Equal(suite.T(), res.Item.Id, res2.Item.Id)
	assert.Equal(suite.T(), res.Item.MerchantId, res2.Item.MerchantId)
	assert.Equal(suite.T(), res.Item.UserId, res2.Item.UserId)
	assert.Equal(suite.T(), res.Item.OriginalName, res2.Item.OriginalName)
	assert.Equal(suite.T(), res.Item.FilePath, res2.Item.FilePath)
	assert.Equal(suite.T(), res.Item.Description, res2.Item.Description)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetMerchantDocument_ErrorNotFound() {
	res := &billingpb.GetMerchantDocumentResponse{}
	err := suite.service.GetMerchantDocument(context.Background(), &billingpb.GetMerchantDocumentRequest{
		Id: "id",
	}, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), errorMerchantDocumentNotFound, res.Message)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetMerchantDocument_ErrorAccessDenied() {
	suite.merchant.Status = billingpb.MerchantStatusPending
	err := suite.service.merchantRepository.Update(context.Background(), suite.merchant)
	assert.NoError(suite.T(), err)

	res := &billingpb.AddMerchantDocumentResponse{}
	err = suite.service.AddMerchantDocument(context.Background(), &billingpb.MerchantDocument{
		MerchantId: suite.merchant.Id,
		UserId:     primitive.NewObjectID().Hex(),
	}, res)
	assert.NoError(suite.T(), err)

	res2 := &billingpb.GetMerchantDocumentResponse{}
	err = suite.service.GetMerchantDocument(context.Background(), &billingpb.GetMerchantDocumentRequest{
		Id:         res.Item.Id,
		MerchantId: primitive.NewObjectID().Hex(),
	}, res2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusForbidden, res2.Status)
	assert.Equal(suite.T(), errorMerchantDocumentAccessDenied, res2.Message)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetMerchantDocuments_Ok() {
	suite.merchant.Status = billingpb.MerchantStatusPending
	err := suite.service.merchantRepository.Update(context.Background(), suite.merchant)
	assert.NoError(suite.T(), err)

	res := &billingpb.AddMerchantDocumentResponse{}
	err = suite.service.AddMerchantDocument(context.Background(), &billingpb.MerchantDocument{
		MerchantId: suite.merchant.Id,
		UserId:     primitive.NewObjectID().Hex(),
	}, res)
	assert.NoError(suite.T(), err)

	res2 := &billingpb.GetMerchantDocumentsResponse{}
	err = suite.service.GetMerchantDocuments(context.Background(), &billingpb.GetMerchantDocumentsRequest{
		MerchantId: res.Item.MerchantId,
		Offset:     0,
		Limit:      1,
	}, res2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Equal(suite.T(), int64(1), res2.Count)
	assert.NotEmpty(suite.T(), res2.List)
	assert.Len(suite.T(), res2.List, 1)
	assert.Equal(suite.T(), res.Item.Id, res2.List[0].Id)
	assert.Equal(suite.T(), res.Item.MerchantId, res2.List[0].MerchantId)
	assert.Equal(suite.T(), res.Item.UserId, res2.List[0].UserId)
	assert.Equal(suite.T(), res.Item.OriginalName, res2.List[0].OriginalName)
	assert.Equal(suite.T(), res.Item.FilePath, res2.List[0].FilePath)
	assert.Equal(suite.T(), res.Item.Description, res2.List[0].Description)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetMerchantDocuments_OkNotFound() {
	suite.merchant.Status = billingpb.MerchantStatusPending
	err := suite.service.merchantRepository.Update(context.Background(), suite.merchant)
	assert.NoError(suite.T(), err)

	res := &billingpb.AddMerchantDocumentResponse{}
	err = suite.service.AddMerchantDocument(context.Background(), &billingpb.MerchantDocument{
		MerchantId: suite.merchant.Id,
		UserId:     primitive.NewObjectID().Hex(),
	}, res)
	assert.NoError(suite.T(), err)

	res2 := &billingpb.GetMerchantDocumentsResponse{}
	err = suite.service.GetMerchantDocuments(context.Background(), &billingpb.GetMerchantDocumentsRequest{
		MerchantId: primitive.NewObjectID().Hex(),
		Offset:     0,
		Limit:      1,
	}, res2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
	assert.Empty(suite.T(), res2.List)
}
