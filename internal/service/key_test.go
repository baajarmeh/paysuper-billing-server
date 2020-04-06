package service

import (
	"context"
	"errors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	errors2 "github.com/paysuper/paysuper-billing-server/pkg/errors"
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
	"time"
)

type KeyTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger
	cache   database.CacheInterface
}

func Test_Key(t *testing.T) {
	suite.Run(t, new(KeyTestSuite))
}

func (suite *KeyTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

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

	redisdb := mocks.NewTestRedis()
	suite.cache, err = database.NewCacheRedis(redisdb, "cache")
	suite.service = NewBillingService(
		db,
		cfg,
		mocks.NewGeoIpServiceTestOk(),
		mocks.NewRepositoryServiceOk(),
		mocks.NewTaxServiceOkMock(),
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
}

func (suite *KeyTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *KeyTestSuite) TestKey_Insert_Ok() {
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		Code:         "code",
	}))
}

func (suite *KeyTestSuite) TestKey_Insert_Error_Duplicate() {
	key := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		OrderId:      primitive.NewObjectID().Hex(),
		Code:         "code",
	}
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, key))

	key.Id = primitive.NewObjectID().Hex()
	assert.Errorf(suite.T(), suite.service.keyRepository.Insert(ctx, key), "duplicate key error collection")
}

func (suite *KeyTestSuite) TestKey_GetById_Ok() {
	key := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		OrderId:      primitive.NewObjectID().Hex(),
		Code:         "code",
	}
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, key))

	k, err := suite.service.keyRepository.GetById(ctx, key.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), key.Id, k.Id)
	assert.Equal(suite.T(), key.PlatformId, k.PlatformId)
	assert.Equal(suite.T(), key.KeyProductId, k.KeyProductId)
	assert.Equal(suite.T(), key.OrderId, k.OrderId)
	assert.Equal(suite.T(), key.Code, k.Code)
}

func (suite *KeyTestSuite) TestKey_GetById_Error_NotFound() {
	_, err := suite.service.keyRepository.GetById(ctx, primitive.NewObjectID().Hex())
	assert.Error(suite.T(), err)
}

func (suite *KeyTestSuite) TestKey_ReserveKey_Ok() {
	key := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		Code:         "code1",
	}
	duration := int32(3)
	orderId := primitive.NewObjectID().Hex()
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, key))

	now := time.Now().UTC()
	k, err := suite.service.keyRepository.ReserveKey(ctx, key.KeyProductId, key.PlatformId, orderId, duration)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), key.Id, k.Id)
	assert.Equal(suite.T(), key.PlatformId, k.PlatformId)
	assert.Equal(suite.T(), key.KeyProductId, k.KeyProductId)
	assert.Equal(suite.T(), orderId, k.OrderId)
	assert.Equal(suite.T(), key.Code, k.Code)

	redeemedAt, err := ptypes.Timestamp(k.RedeemedAt)
	if err != nil {
		assert.FailNow(suite.T(), "Invalid redeemed at")
	}
	assert.Equal(suite.T(), "0001-01-01 00:00:00 +0000 UTC", redeemedAt.String())

	reservedTo, err := ptypes.Timestamp(k.ReservedTo)
	if err != nil {
		assert.FailNow(suite.T(), "Invalid reserved to")
	}
	assert.Equal(
		suite.T(),
		now.Add(time.Second*time.Duration(duration)).Format("2006-01-02T15:04:05"),
		reservedTo.Format("2006-01-02T15:04:05"),
	)
}

func (suite *KeyTestSuite) TestKey_ReserveKey_Error_NotFound() {
	key := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		Code:         "code1",
	}
	orderId := primitive.NewObjectID().Hex()

	_, err := suite.service.keyRepository.ReserveKey(ctx, key.KeyProductId, key.PlatformId, orderId, 3)
	assert.Error(suite.T(), err)
}

func (suite *KeyTestSuite) TestKey_ReserveKey_Error_NotFree() {
	key := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		OrderId:      primitive.NewObjectID().Hex(),
		Code:         "code1",
	}
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, key))

	_, err := suite.service.keyRepository.ReserveKey(ctx, key.KeyProductId, key.PlatformId, key.OrderId, 3)
	assert.Error(suite.T(), err)
}

func (suite *KeyTestSuite) TestKey_CancelById_Ok() {
	key := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		Code:         "code1",
	}
	orderId := primitive.NewObjectID().Hex()
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, key))

	_, err := suite.service.keyRepository.ReserveKey(ctx, key.KeyProductId, key.PlatformId, orderId, 3)
	assert.NoError(suite.T(), err)

	k, err := suite.service.keyRepository.CancelById(ctx, key.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), key.Id, k.Id)
	assert.Equal(suite.T(), key.PlatformId, k.PlatformId)
	assert.Equal(suite.T(), key.KeyProductId, k.KeyProductId)
	assert.Equal(suite.T(), key.Code, k.Code)
	assert.Empty(suite.T(), k.OrderId)

	reservedTo, err := ptypes.Timestamp(k.ReservedTo)
	if err != nil {
		assert.FailNow(suite.T(), "Invalid reserved to")
	}
	assert.Equal(suite.T(), "0001-01-01 00:00:00 +0000 UTC", reservedTo.String())
}

func (suite *KeyTestSuite) TestKey_CancelById_Error_NotFound() {
	_, err := suite.service.keyRepository.CancelById(ctx, primitive.NewObjectID().Hex())
	assert.Error(suite.T(), err)
}

func (suite *KeyTestSuite) TestKey_FinishRedeemById_Ok() {
	key := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		Code:         "code1",
	}
	orderId := primitive.NewObjectID().Hex()
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, key))

	_, err := suite.service.keyRepository.ReserveKey(ctx, key.KeyProductId, key.PlatformId, orderId, 3)
	assert.NoError(suite.T(), err)

	k, err := suite.service.keyRepository.FinishRedeemById(ctx, key.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), key.Id, k.Id)
	assert.Equal(suite.T(), key.PlatformId, k.PlatformId)
	assert.Equal(suite.T(), key.KeyProductId, k.KeyProductId)
	assert.Equal(suite.T(), key.Code, k.Code)
	assert.Equal(suite.T(), orderId, k.OrderId)

	redeemedAt, err := ptypes.Timestamp(k.RedeemedAt)
	if err != nil {
		assert.FailNow(suite.T(), "Invalid redeemed at")
	}
	assert.Equal(
		suite.T(),
		time.Now().UTC().Format("2006-01-02T15:04:05"),
		redeemedAt.Format("2006-01-02T15:04:05"),
	)
}

func (suite *KeyTestSuite) TestKey_FinishRedeemById_Error_NotFound() {
	_, err := suite.service.keyRepository.FinishRedeemById(ctx, primitive.NewObjectID().Hex())
	assert.Error(suite.T(), err)
}

func (suite *KeyTestSuite) TestKey_CountKeysByProductPlatform_Ok() {
	platformId := "steam"
	keyProductId := primitive.NewObjectID().Hex()

	cnt, err := suite.service.keyRepository.CountKeysByProductPlatform(ctx, keyProductId, platformId)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 0, cnt)

	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   platformId,
		KeyProductId: keyProductId,
		Code:         "code1",
	}))
	cnt, err = suite.service.keyRepository.CountKeysByProductPlatform(ctx, keyProductId, platformId)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 1, cnt)

	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   platformId,
		KeyProductId: keyProductId,
		Code:         "code2",
		OrderId:      primitive.NewObjectID().Hex(),
	}))
	cnt, err = suite.service.keyRepository.CountKeysByProductPlatform(ctx, keyProductId, platformId)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 1, cnt)
}

func (suite *KeyTestSuite) TestKey_GetAvailableKeysCount_Ok() {
	req := &billingpb.GetPlatformKeyCountRequest{
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetPlatformKeyCountResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CountKeysByProductPlatform", mock2.Anything, req.KeyProductId, req.PlatformId).Return(int64(1), nil)
	suite.service.keyRepository = kr

	kp := &mocks.KeyProductRepositoryInterface{}
	kp.On("GetById", mock2.Anything, req.KeyProductId).Return(&billingpb.KeyProduct{MerchantId: req.MerchantId}, nil)
	suite.service.keyProductRepository = kp

	err := suite.service.GetAvailableKeysCount(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int32(1), res.Count)
}

func (suite *KeyTestSuite) TestKey_GetAvailableKeysCount_Error_KeyProductNotFound() {
	req := &billingpb.GetPlatformKeyCountRequest{
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetPlatformKeyCountResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CountKeysByProductPlatform", req.KeyProductId, req.PlatformId).Return(0, errors.New("not found"))
	suite.service.keyRepository = kr

	err := suite.service.GetAvailableKeysCount(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), keyProductNotFound, res.Message)
}

func (suite *KeyTestSuite) TestKey_GetAvailableKeysCount_Error_MerchantMismatch() {
	req := &billingpb.GetPlatformKeyCountRequest{
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		MerchantId:   primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetPlatformKeyCountResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CountKeysByProductPlatform", mock2.Anything, req.KeyProductId, req.PlatformId).Return(0, errors.New("not found"))
	suite.service.keyRepository = kr

	kp := &mocks.KeyProductRepositoryInterface{}
	kp.On("GetById", mock2.Anything, req.KeyProductId).Return(&billingpb.KeyProduct{MerchantId: primitive.NewObjectID().Hex()}, nil)
	suite.service.keyProductRepository = kp

	err := suite.service.GetAvailableKeysCount(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), keyProductMerchantMismatch, res.Message)
}

func (suite *KeyTestSuite) TestKey_GetAvailableKeysCount_Error_NotFound() {
	req := &billingpb.GetPlatformKeyCountRequest{
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		MerchantId:   primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetPlatformKeyCountResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CountKeysByProductPlatform", mock2.Anything, req.KeyProductId, req.PlatformId).
		Return(int64(0), errors.New("not found"))
	suite.service.keyRepository = kr

	kp := &mocks.KeyProductRepositoryInterface{}
	kp.On("GetById", mock2.Anything, req.KeyProductId).Return(&billingpb.KeyProduct{MerchantId: req.MerchantId}, nil)
	suite.service.keyProductRepository = kp

	err := suite.service.GetAvailableKeysCount(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), errors2.KeyErrorNotFound, res.Message)
}

func (suite *KeyTestSuite) TestKey_GetKeyByID_Ok() {
	req := &billingpb.KeyForOrderRequest{
		KeyId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetKeyForOrderRequestResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("GetById", mock2.Anything, req.KeyId).Return(&billingpb.Key{}, nil)
	suite.service.keyRepository = kr

	err := suite.service.GetKeyByID(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
}

func (suite *KeyTestSuite) TestKey_GetKeyByID_Error_NotFound() {
	req := &billingpb.KeyForOrderRequest{
		KeyId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetKeyForOrderRequestResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("GetById", mock2.Anything, req.KeyId).Return(nil, errors.New("not found"))
	suite.service.keyRepository = kr

	err := suite.service.GetKeyByID(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), errors2.KeyErrorNotFound, res.Message)
}

func (suite *KeyTestSuite) TestKey_ReserveKeyForOrder_Ok() {
	req := &billingpb.PlatformKeyReserveRequest{
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		OrderId:      primitive.NewObjectID().Hex(),
		Ttl:          3,
	}
	res := billingpb.PlatformKeyReserveResponse{}
	keyId := primitive.NewObjectID().Hex()

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("ReserveKey", mock2.Anything, req.KeyProductId, req.PlatformId, req.OrderId, req.Ttl).Return(&billingpb.Key{Id: keyId}, nil)
	suite.service.keyRepository = kr

	err := suite.service.ReserveKeyForOrder(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), keyId, res.KeyId)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
}

func (suite *KeyTestSuite) TestKey_ReserveKeyForOrder_Error_Reserve() {
	req := &billingpb.PlatformKeyReserveRequest{
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		OrderId:      primitive.NewObjectID().Hex(),
		Ttl:          3,
	}
	res := billingpb.PlatformKeyReserveResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("ReserveKey", mock2.Anything, req.KeyProductId, req.PlatformId, req.OrderId, req.Ttl).Return(nil, errors.New("error"))
	suite.service.keyRepository = kr

	err := suite.service.ReserveKeyForOrder(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, res.Status)
	assert.Equal(suite.T(), errors2.KeyErrorReserve, res.Message)
}

func (suite *KeyTestSuite) TestKey_FinishRedeemKeyForOrder_Ok() {
	req := &billingpb.KeyForOrderRequest{
		KeyId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetKeyForOrderRequestResponse{}
	key := &billingpb.Key{
		Id: primitive.NewObjectID().Hex(),
	}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("FinishRedeemById", mock2.Anything, req.KeyId).Return(key, nil)
	suite.service.keyRepository = kr

	err := suite.service.FinishRedeemKeyForOrder(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), key.Id, res.Key.Id)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
}

func (suite *KeyTestSuite) TestKey_FinishRedeemKeyForOrder_Error_NotFound() {
	req := &billingpb.KeyForOrderRequest{
		KeyId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.GetKeyForOrderRequestResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("FinishRedeemById", mock2.Anything, req.KeyId).Return(nil, errors.New("not found"))
	suite.service.keyRepository = kr

	err := suite.service.FinishRedeemKeyForOrder(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), errors2.KeyErrorFinish, res.Message)
}

func (suite *KeyTestSuite) TestKey_CancelRedeemKeyForOrder_Ok() {
	req := &billingpb.KeyForOrderRequest{
		KeyId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.EmptyResponseWithStatus{}
	key := &billingpb.Key{
		Id: primitive.NewObjectID().Hex(),
	}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CancelById", mock2.Anything, req.KeyId).Return(key, nil)
	suite.service.keyRepository = kr

	err := suite.service.CancelRedeemKeyForOrder(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
}

func (suite *KeyTestSuite) TestKey_CancelRedeemKeyForOrder_Error_NotFound() {
	req := &billingpb.KeyForOrderRequest{
		KeyId: primitive.NewObjectID().Hex(),
	}
	res := billingpb.EmptyResponseWithStatus{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CancelById", mock2.Anything, req.KeyId).Return(nil, errors.New("not found"))
	suite.service.keyRepository = kr

	err := suite.service.CancelRedeemKeyForOrder(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), errors2.KeyErrorCanceled, res.Message)
}

func (suite *KeyTestSuite) TestKey_UploadKeysFile_Ok() {
	req := &billingpb.PlatformKeysFileRequest{
		KeyProductId: primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		File:         []byte{},
	}
	res := billingpb.PlatformKeysFileResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CountKeysByProductPlatform", mock2.Anything, req.KeyProductId, req.PlatformId).Return(int64(1), nil)
	kr.On("Insert", mock2.Anything).Return(nil)
	suite.service.keyRepository = kr

	err := suite.service.UploadKeysFile(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int32(1), res.TotalCount)
	assert.Equal(suite.T(), int32(0), res.KeysProcessed)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, res.Status)
}

func (suite *KeyTestSuite) TestKey_UploadKeysFile_Error_CountKeysByProductPlatform() {
	req := &billingpb.PlatformKeysFileRequest{
		KeyProductId: primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		File:         []byte{},
	}
	res := billingpb.PlatformKeysFileResponse{}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("CountKeysByProductPlatform", mock2.Anything, req.KeyProductId, req.PlatformId).
		Return(int64(0), errors.New("not found"))
	kr.On("Insert", mock2.Anything, mock2.Anything).Return(nil)
	suite.service.keyRepository = kr

	err := suite.service.UploadKeysFile(context.TODO(), req, &res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, res.Status)
	assert.Equal(suite.T(), errors2.KeyErrorNotFound, res.Message)
}

func (suite *KeyTestSuite) TestKey_KeyDaemonProcess_Ok() {
	keys := []*billingpb.Key{{Id: primitive.NewObjectID().Hex()}}
	kr := &mocks.KeyRepositoryInterface{}
	kr.On("FindUnfinished", mock2.Anything).Return(keys, nil)
	kr.On("CancelById", mock2.Anything, keys[0].Id).Return(&billingpb.Key{}, nil)
	suite.service.keyRepository = kr

	count, err := suite.service.KeyDaemonProcess(ctx)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, count)
}

func (suite *KeyTestSuite) TestKey_KeyDaemonProcess_Error_FindUnfinished() {
	kr := &mocks.KeyRepositoryInterface{}
	kr.On("FindUnfinished", mock2.Anything).Return(nil, errors.New("not found"))
	suite.service.keyRepository = kr

	count, err := suite.service.KeyDaemonProcess(ctx)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

func (suite *KeyTestSuite) TestKey_KeyDaemonProcess_Error_CancelById() {
	keys := []*billingpb.Key{{Id: primitive.NewObjectID().Hex()}}

	kr := &mocks.KeyRepositoryInterface{}
	kr.On("FindUnfinished", mock2.Anything).Return(keys, nil)
	kr.On("CancelById", mock2.Anything, keys[0].Id).Return(nil, errors.New("not found"))
	suite.service.keyRepository = kr

	count, _ := suite.service.KeyDaemonProcess(context.TODO())
	assert.Equal(suite.T(), 0, count)
}

func (suite *KeyTestSuite) TestKey_FindUnfinished_Ok() {
	reserveExpireTime, _ := ptypes.TimestampProto(time.Now().AddDate(0, 0, -1))
	reserveNoExpireTime, _ := ptypes.TimestampProto(time.Now().AddDate(0, 0, 1))

	keyReserveExpire := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "steam",
		KeyProductId: primitive.NewObjectID().Hex(),
		Code:         "code1",
		ReservedTo:   reserveExpireTime,
	}
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, keyReserveExpire))

	keyReserveNoExpire := &billingpb.Key{
		Id:           primitive.NewObjectID().Hex(),
		PlatformId:   "gog",
		KeyProductId: primitive.NewObjectID().Hex(),
		Code:         "code1",
		ReservedTo:   reserveNoExpireTime,
	}
	assert.NoError(suite.T(), suite.service.keyRepository.Insert(ctx, keyReserveNoExpire))

	keys, err := suite.service.keyRepository.FindUnfinished(ctx)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), keys, 1)
	assert.Equal(suite.T(), keyReserveExpire.Id, keys[0].Id)
}
