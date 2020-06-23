package service

import (
	"context"
	"errors"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type ZipCodeTestSuite struct {
	suite.Suite
	service *Service
}

func Test_ZipCode(t *testing.T) {
	suite.Run(t, new(ZipCodeTestSuite))
}

func (suite *ZipCodeTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	db, err := mongodb.NewDatabase()

	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	redisdb := mocks.NewTestRedis()
	cache, err := database.NewCacheRedis(redisdb, "cache")

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
		cache,
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
}

func (suite *ZipCodeTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *ZipCodeTestSuite) TestZipCode_FindByZipCode_NoneUs() {
	req := &billingpb.FindByZipCodeRequest{
		Zip:     "98",
		Country: "UA",
	}

	rsp := &billingpb.FindByZipCodeResponse{}
	err := suite.service.FindByZipCode(context.Background(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), int32(0), rsp.Count)
	assert.Len(suite.T(), rsp.Items, 0)
}

func (suite *ZipCodeTestSuite) TestZipCode_FindByZipCode_ErrorByCount() {
	req := &billingpb.FindByZipCodeRequest{
		Zip:     "98",
		Country: "US",
	}

	rep := &mocks.ZipCodeRepositoryInterface{}
	rep.On("CountByZip", mock.Anything, req.Zip, req.Country).Return(int64(0), errors.New("error"))
	suite.service.zipCodeRepository = rep

	rsp := &billingpb.FindByZipCodeResponse{}
	err := suite.service.FindByZipCode(context.Background(), req, rsp)
	assert.Equal(suite.T(), orderErrorUnknown, err)
	assert.Equal(suite.T(), int32(0), rsp.Count)
	assert.Len(suite.T(), rsp.Items, 0)
}

func (suite *ZipCodeTestSuite) TestZipCode_FindByZipCode_EmptyCount() {
	req := &billingpb.FindByZipCodeRequest{
		Zip:     "98",
		Country: "US",
	}

	rep := &mocks.ZipCodeRepositoryInterface{}
	rep.On("CountByZip", mock.Anything, req.Zip, req.Country).Return(int64(0), nil)
	suite.service.zipCodeRepository = rep

	rsp := &billingpb.FindByZipCodeResponse{}
	err := suite.service.FindByZipCode(context.Background(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), int32(0), rsp.Count)
	assert.Len(suite.T(), rsp.Items, 0)
}

func (suite *ZipCodeTestSuite) TestZipCode_FindByZipCode_ErrorByFind() {
	req := &billingpb.FindByZipCodeRequest{
		Zip:     "98",
		Country: "US",
		Limit:   1,
		Offset:  0,
	}

	rep := &mocks.ZipCodeRepositoryInterface{}
	rep.On("CountByZip", mock.Anything, req.Zip, req.Country).Return(int64(1), nil)
	rep.On("FindByZipAndCountry", mock.Anything, req.Zip, req.Country, req.Offset, req.Limit).Return(nil, errors.New("error"))
	suite.service.zipCodeRepository = rep

	rsp := &billingpb.FindByZipCodeResponse{}
	err := suite.service.FindByZipCode(context.Background(), req, rsp)
	assert.Equal(suite.T(), orderErrorUnknown, err)
	assert.Equal(suite.T(), int32(0), rsp.Count)
	assert.Len(suite.T(), rsp.Items, 0)
}

func (suite *ZipCodeTestSuite) TestZipCode_FindByZipCode_Ok() {
	req := &billingpb.FindByZipCodeRequest{
		Zip:     "98",
		Country: "US",
		Limit:   1,
		Offset:  0,
	}

	rep := &mocks.ZipCodeRepositoryInterface{}
	rep.On("CountByZip", mock.Anything, req.Zip, req.Country).Return(int64(1), nil)
	rep.On("FindByZipAndCountry", mock.Anything, req.Zip, req.Country, req.Offset, req.Limit).
		Return([]*billingpb.ZipCode{{Zip: "123"}}, nil)
	suite.service.zipCodeRepository = rep

	rsp := &billingpb.FindByZipCodeResponse{}
	err := suite.service.FindByZipCode(context.Background(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int32(1), rsp.Count)
	assert.Len(suite.T(), rsp.Items, 1)
}
