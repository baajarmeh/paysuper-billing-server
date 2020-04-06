package service

import (
	"context"
	"github.com/go-redis/redis"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"net/http"
	"testing"
	"time"
)

type MerchantBalanceTestSuite struct {
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

func Test_MerchantBalance(t *testing.T) {
	suite.Run(t, new(MerchantBalanceTestSuite))
}

func (suite *MerchantBalanceTestSuite) SetupTest() {
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
	suite.httpClient = mocks.NewClientStatusOk()
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

func (suite *MerchantBalanceTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_GetMerchantBalance_Ok_AutoCreateBalance() {
	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	req := &billingpb.GetMerchantBalanceRequest{
		MerchantId: suite.merchant.Id,
	}

	res := &billingpb.GetMerchantBalanceResponse{}

	err := suite.service.GetMerchantBalance(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), res.Item.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), res.Item.Currency, suite.merchant.GetPayoutCurrency())
	assert.Equal(suite.T(), res.Item.Debit, float64(0))
	assert.Equal(suite.T(), res.Item.Credit, float64(0))
	assert.Equal(suite.T(), res.Item.RollingReserve, float64(0))
	assert.Equal(suite.T(), res.Item.Total, float64(0))

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_GetMerchantBalance_Ok_NoAutoCreateBalance() {
	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	mb, err := suite.service.updateMerchantBalance(ctx, suite.merchant.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mb.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mb.Currency, suite.merchant.GetPayoutCurrency())
	assert.Equal(suite.T(), mb.Debit, float64(0))
	assert.Equal(suite.T(), mb.Credit, float64(0))
	assert.Equal(suite.T(), mb.RollingReserve, float64(0))
	assert.Equal(suite.T(), mb.Total, float64(0))

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)

	req := &billingpb.GetMerchantBalanceRequest{
		MerchantId: suite.merchant.Id,
	}

	res := &billingpb.GetMerchantBalanceResponse{}

	err = suite.service.GetMerchantBalance(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)

	// ugly cheat, because Nanos are slightly differs
	// example:
	// mb:       seconds: 1568218155 nanos: 740067400
	// res.Item: seconds: 1568218155 nanos: 740000000
	// wtf?
	mb.CreatedAt.Nanos = 0
	res.Item.CreatedAt.Nanos = 0

	assert.Equal(suite.T(), res.Item, mb)

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_GetMerchantBalance_Failed_MerchantNotFound() {
	merchantId = primitive.NewObjectID().Hex()

	count := suite.mbRecordsCount(merchantId, "")
	assert.EqualValues(suite.T(), count, 0)

	req := &billingpb.GetMerchantBalanceRequest{
		MerchantId: merchantId,
	}

	res := &billingpb.GetMerchantBalanceResponse{}
	err := suite.service.GetMerchantBalance(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, res.Status)
	assert.Equal(suite.T(), merchantErrorNotFound, res.Message)

	count = suite.mbRecordsCount(merchantId, "")
	assert.EqualValues(suite.T(), count, 0)
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_GetMerchantBalance_Failed_NoPayoutCurrency() {
	count := suite.mbRecordsCount(suite.merchant2.Id, suite.merchant2.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	req := &billingpb.GetMerchantBalanceRequest{
		MerchantId: suite.merchant2.Id,
	}

	res := &billingpb.GetMerchantBalanceResponse{}

	err := suite.service.GetMerchantBalance(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusSystemError)
	assert.Equal(suite.T(), res.Message, errorMerchantPayoutCurrencyNotSet)

	count = suite.mbRecordsCount(suite.merchant2.Id, suite.merchant2.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_updateMerchantBalance_Failed_MerchantNotFound() {
	merchantId = primitive.NewObjectID().Hex()
	count := suite.mbRecordsCount(merchantId, "")
	assert.EqualValues(suite.T(), count, 0)

	mb, err := suite.service.updateMerchantBalance(ctx, merchantId)
	assert.EqualError(suite.T(), err, "merchant with specified identifier not found")
	assert.Nil(suite.T(), mb)

	count = suite.mbRecordsCount(merchantId, "")
	assert.EqualValues(suite.T(), count, 0)
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_updateMerchantBalance_Failed_NoPayoutCurrency() {
	count := suite.mbRecordsCount(suite.merchant2.Id, suite.merchant2.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	mb, err := suite.service.updateMerchantBalance(ctx, suite.merchant2.Id)
	assert.EqualError(suite.T(), err, errorMerchantPayoutCurrencyNotSet.Error())
	assert.Nil(suite.T(), mb)

	count = suite.mbRecordsCount(suite.merchant2.Id, suite.merchant2.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_updateMerchantBalance_Ok() {

	report := &billingpb.RoyaltyReport{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: suite.merchant.Id,
		Totals: &billingpb.RoyaltyReportTotals{
			TransactionsCount: 10,
			PayoutAmount:      1234.5,
			VatAmount:         10,
			FeeAmount:         5,
		},
		Status:         billingpb.RoyaltyReportStatusAccepted,
		CreatedAt:      ptypes.TimestampNow(),
		PeriodFrom:     ptypes.TimestampNow(),
		PeriodTo:       ptypes.TimestampNow(),
		AcceptExpireAt: ptypes.TimestampNow(),
		Currency:       suite.merchant.GetPayoutCurrency(),
	}
	err := suite.service.royaltyReportRepository.Insert(ctx, report, "", pkg.RoyaltyReportChangeSourceAuto)
	assert.NoError(suite.T(), err)

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -480))
	assert.NoError(suite.T(), err, "Generate PayoutDocument date failed")

	payout := &billingpb.PayoutDocument{
		Id:                 primitive.NewObjectID().Hex(),
		MerchantId:         suite.merchant.Id,
		SourceId:           []string{primitive.NewObjectID().Hex()},
		TotalFees:          1000,
		Balance:            1000,
		Currency:           "RUB",
		Status:             pkg.PayoutDocumentStatusPending,
		Description:        "test payout document",
		Destination:        suite.merchant.Banking,
		CreatedAt:          date,
		UpdatedAt:          ptypes.TimestampNow(),
		ArrivalDate:        ptypes.TimestampNow(),
		Transaction:        "",
		FailureTransaction: "",
		FailureMessage:     "",
		FailureCode:        "",
	}
	err = suite.service.payoutRepository.Insert(ctx, payout, "127.0.0.1", payoutChangeSourceAdmin)
	assert.NoError(suite.T(), err)

	ae1 := &billingpb.AccountingEntry{
		Id:     primitive.NewObjectID().Hex(),
		Type:   pkg.AccountingEntryTypeMerchantRollingReserveCreate,
		Object: pkg.ObjectTypeBalanceTransaction,
		Source: &billingpb.AccountingEntrySource{
			Type: "merchant",
			Id:   suite.merchant.Id,
		},
		MerchantId: suite.merchant.Id,
		Amount:     150,
		Currency:   "RUB",
		CreatedAt:  ptypes.TimestampNow(),
	}

	ae2 := &billingpb.AccountingEntry{
		Id:     primitive.NewObjectID().Hex(),
		Type:   pkg.AccountingEntryTypeMerchantRollingReserveRelease,
		Object: pkg.ObjectTypeBalanceTransaction,
		Source: &billingpb.AccountingEntrySource{
			Type: "merchant",
			Id:   suite.merchant.Id,
		},
		MerchantId: suite.merchant.Id,
		Amount:     50,
		Currency:   "RUB",
		CreatedAt:  ptypes.TimestampNow(),
	}

	accountingEntries := []*billingpb.AccountingEntry{ae1, ae2}
	err = suite.service.accountingRepository.MultipleInsert(ctx, accountingEntries)
	assert.NoError(suite.T(), err)

	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	mb, err := suite.service.updateMerchantBalance(ctx, suite.merchant.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mb.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mb.Currency, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), mb.Debit, 1234.5)
	assert.EqualValues(suite.T(), mb.Credit, 1000)
	assert.EqualValues(suite.T(), mb.RollingReserve, 100)
	assert.EqualValues(suite.T(), mb.Total, 134.5)

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)
}

func (suite *MerchantBalanceTestSuite) TestMerchantBalance_getRollingReserveForBalance_Ok() {

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -480))
	assert.NoError(suite.T(), err, "Generate PayoutDocument date failed")

	payout := &billingpb.PayoutDocument{
		Id:                 primitive.NewObjectID().Hex(),
		MerchantId:         suite.merchant.Id,
		SourceId:           []string{primitive.NewObjectID().Hex()},
		TotalFees:          1000,
		Balance:            1000,
		Currency:           "RUB",
		Status:             pkg.PayoutDocumentStatusPending,
		Description:        "test payout document",
		Destination:        suite.merchant.Banking,
		CreatedAt:          date,
		UpdatedAt:          ptypes.TimestampNow(),
		ArrivalDate:        ptypes.TimestampNow(),
		Transaction:        "",
		FailureTransaction: "",
		FailureMessage:     "",
		FailureCode:        "",
	}
	err = suite.service.payoutRepository.Insert(ctx, payout, "127.0.0.1", payoutChangeSourceAdmin)
	assert.NoError(suite.T(), err)

	rr, err := suite.service.getRollingReserveForBalance(ctx, suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), rr, float64(0))
}

// 1. trigger for merchant accepting royalty report
func (suite *MerchantBalanceTestSuite) TestMerchantBalance_UpdateBalanceTriggeringOk_1() {
	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	report := &billingpb.RoyaltyReport{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: suite.merchant.Id,
		Totals: &billingpb.RoyaltyReportTotals{
			TransactionsCount: 10,
			PayoutAmount:      1234.5,
			VatAmount:         10,
			FeeAmount:         5,
		},
		Status:         billingpb.RoyaltyReportStatusPending,
		CreatedAt:      ptypes.TimestampNow(),
		PeriodFrom:     ptypes.TimestampNow(),
		PeriodTo:       ptypes.TimestampNow(),
		AcceptExpireAt: ptypes.TimestampNow(),
		Currency:       suite.merchant.GetPayoutCurrency(),
	}
	err := suite.service.royaltyReportRepository.Insert(ctx, report, "", pkg.RoyaltyReportChangeSourceAuto)
	assert.NoError(suite.T(), err)

	req1 := &billingpb.MerchantReviewRoyaltyReportRequest{
		ReportId:   report.Id,
		IsAccepted: true,
		Ip:         "127.0.0.1",
		MerchantId: suite.merchant.Id,
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.MerchantReviewRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)

	// control

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)

	mbReq := &billingpb.GetMerchantBalanceRequest{MerchantId: suite.merchant.Id}
	mbRes := &billingpb.GetMerchantBalanceResponse{}
	err = suite.service.GetMerchantBalance(context.TODO(), mbReq, mbRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mbRes.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), mbRes.Item.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mbRes.Item.Currency, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), 1234.5, mbRes.Item.Debit)
	assert.EqualValues(suite.T(), 0, mbRes.Item.Credit)
	assert.EqualValues(suite.T(), 0, mbRes.Item.RollingReserve)
	assert.EqualValues(suite.T(), 1234.5, mbRes.Item.Total)
}

// 2. Triggering on payout status change
func (suite *MerchantBalanceTestSuite) TestMerchantBalance_UpdateBalanceTriggeringOk_3() {

	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	report := &billingpb.RoyaltyReport{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: suite.merchant.Id,
		Totals: &billingpb.RoyaltyReportTotals{
			TransactionsCount: 100,
			PayoutAmount:      alreadyPaidRoyalty,
			VatAmount:         100,
			FeeAmount:         50,
		},
		Status:         billingpb.RoyaltyReportStatusAccepted,
		CreatedAt:      ptypes.TimestampNow(),
		PeriodFrom:     ptypes.TimestampNow(),
		PeriodTo:       ptypes.TimestampNow(),
		AcceptExpireAt: ptypes.TimestampNow(),
		Currency:       suite.merchant.GetPayoutCurrency(),
	}

	err := suite.service.royaltyReportRepository.Insert(ctx, report, "", pkg.RoyaltyReportChangeSourceAuto)
	assert.NoError(suite.T(), err)

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -480))
	assert.NoError(suite.T(), err, "Generate PayoutDocument date failed")

	payout := &billingpb.PayoutDocument{
		Id:                 primitive.NewObjectID().Hex(),
		MerchantId:         suite.merchant.Id,
		SourceId:           []string{report.Id},
		TotalFees:          1000,
		Balance:            1000,
		Currency:           "RUB",
		Status:             pkg.PayoutDocumentStatusPending,
		Description:        "test payout document",
		Destination:        suite.merchant.Banking,
		CreatedAt:          date,
		UpdatedAt:          ptypes.TimestampNow(),
		ArrivalDate:        ptypes.TimestampNow(),
		Transaction:        "",
		FailureTransaction: "",
		FailureMessage:     "",
		FailureCode:        "",
	}
	err = suite.service.payoutRepository.Insert(ctx, payout, "127.0.0.1", payoutChangeSourceAdmin)
	assert.NoError(suite.T(), err)

	req3 := &billingpb.UpdatePayoutDocumentRequest{
		PayoutDocumentId: payout.Id,
		Status:           pkg.PayoutDocumentStatusPaid,
		Transaction:      "transaction123",
		Ip:               "192.168.1.1",
	}

	res3 := &billingpb.PayoutDocumentResponse{}

	err = suite.service.UpdatePayoutDocument(context.TODO(), req3, res3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res3.Status, billingpb.ResponseStatusOk)

	// control

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)

	mbReq := &billingpb.GetMerchantBalanceRequest{MerchantId: suite.merchant.Id}
	mbRes := &billingpb.GetMerchantBalanceResponse{}
	err = suite.service.GetMerchantBalance(context.TODO(), mbReq, mbRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mbRes.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), mbRes.Item.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mbRes.Item.Currency, suite.merchant.GetPayoutCurrency())
	assert.Equal(suite.T(), mbRes.Item.Debit, float64(10432))
	assert.Equal(suite.T(), mbRes.Item.Credit, float64(1000))
	assert.Equal(suite.T(), mbRes.Item.RollingReserve, float64(0))
	assert.Equal(suite.T(), mbRes.Item.Total, float64(9432))
}

// 3. Triggering on rolling reserve create
func (suite *MerchantBalanceTestSuite) TestMerchantBalance_UpdateBalanceTriggeringOk_4() {

	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	req4 := &billingpb.CreateAccountingEntryRequest{
		Type:       pkg.AccountingEntryTypeMerchantRollingReserveCreate,
		MerchantId: suite.merchant.Id,
		Amount:     150,
		Currency:   "RUB",
		Status:     pkg.BalanceTransactionStatusAvailable,
		Date:       time.Now().Unix(),
		Reason:     "unit test",
	}
	rsp4 := &billingpb.CreateAccountingEntryResponse{}
	err := suite.service.CreateAccountingEntry(context.TODO(), req4, rsp4)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp4.Status)
	assert.Empty(suite.T(), rsp4.Message)
	assert.NotNil(suite.T(), rsp4.Item)

	// control

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)

	mbReq := &billingpb.GetMerchantBalanceRequest{MerchantId: suite.merchant.Id}
	mbRes := &billingpb.GetMerchantBalanceResponse{}
	err = suite.service.GetMerchantBalance(context.TODO(), mbReq, mbRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mbRes.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), mbRes.Item.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mbRes.Item.Currency, suite.merchant.GetPayoutCurrency())
	assert.Equal(suite.T(), mbRes.Item.Debit, float64(0))
	assert.Equal(suite.T(), mbRes.Item.Credit, float64(0))
	assert.Equal(suite.T(), mbRes.Item.RollingReserve, float64(150))
	assert.Equal(suite.T(), mbRes.Item.Total, float64(-150))
}

// 4. Triggering on rolling reserve create
func (suite *MerchantBalanceTestSuite) TestMerchantBalance_UpdateBalanceTriggeringOk_5() {

	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	req5 := &billingpb.CreateAccountingEntryRequest{
		Type:       pkg.AccountingEntryTypeMerchantRollingReserveRelease,
		MerchantId: suite.merchant.Id,
		Amount:     50,
		Currency:   "RUB",
		Status:     pkg.BalanceTransactionStatusAvailable,
		Date:       time.Now().Unix(),
		Reason:     "unit test",
	}
	rsp5 := &billingpb.CreateAccountingEntryResponse{}
	err := suite.service.CreateAccountingEntry(context.TODO(), req5, rsp5)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp5.Status)
	assert.Empty(suite.T(), rsp5.Message)
	assert.NotNil(suite.T(), rsp5.Item)

	// control

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)

	mbReq := &billingpb.GetMerchantBalanceRequest{MerchantId: suite.merchant.Id}
	mbRes := &billingpb.GetMerchantBalanceResponse{}
	err = suite.service.GetMerchantBalance(context.TODO(), mbReq, mbRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mbRes.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), mbRes.Item.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mbRes.Item.Currency, suite.merchant.GetPayoutCurrency())
	assert.Equal(suite.T(), mbRes.Item.Debit, float64(0))
	assert.Equal(suite.T(), mbRes.Item.Credit, float64(0))
	assert.Equal(suite.T(), mbRes.Item.RollingReserve, float64(-50))
	assert.Equal(suite.T(), mbRes.Item.Total, float64(50))
}

// 5. trigger on auto-accepting royalty report
func (suite *MerchantBalanceTestSuite) TestMerchantBalance_UpdateBalanceTriggeringOk_6() {

	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -480))
	assert.NoError(suite.T(), err, "Generate PayoutDocument date failed")

	report := &billingpb.RoyaltyReport{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: suite.merchant.Id,
		Totals: &billingpb.RoyaltyReportTotals{
			TransactionsCount: 10,
			PayoutAmount:      1000,
			VatAmount:         10,
			FeeAmount:         5,
		},
		Status:         billingpb.RoyaltyReportStatusPending,
		CreatedAt:      ptypes.TimestampNow(),
		PeriodFrom:     ptypes.TimestampNow(),
		PeriodTo:       ptypes.TimestampNow(),
		AcceptExpireAt: date,
		Currency:       suite.merchant.GetPayoutCurrency(),
	}
	err = suite.service.royaltyReportRepository.Insert(ctx, report, "", pkg.RoyaltyReportChangeSourceAuto)
	assert.NoError(suite.T(), err)

	req6 := &billingpb.EmptyRequest{}
	rsp6 := &billingpb.EmptyResponse{}
	err = suite.service.AutoAcceptRoyaltyReports(context.TODO(), req6, rsp6)
	assert.NoError(suite.T(), err)

	// control

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)

	mbReq := &billingpb.GetMerchantBalanceRequest{MerchantId: suite.merchant.Id}
	mbRes := &billingpb.GetMerchantBalanceResponse{}
	err = suite.service.GetMerchantBalance(context.TODO(), mbReq, mbRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mbRes.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), mbRes.Item.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mbRes.Item.Currency, suite.merchant.GetPayoutCurrency())
	assert.Equal(suite.T(), mbRes.Item.Debit, float64(1000))
	assert.Equal(suite.T(), mbRes.Item.Credit, float64(0))
	assert.Equal(suite.T(), mbRes.Item.RollingReserve, float64(0))
	assert.Equal(suite.T(), mbRes.Item.Total, float64(1000))
}

// 6. trigger on change royalty report with accepting
func (suite *MerchantBalanceTestSuite) TestMerchantBalance_UpdateBalanceTriggeringOk_7() {

	count := suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 0)

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -480))
	assert.NoError(suite.T(), err, "Generate PayoutDocument date failed")

	report := &billingpb.RoyaltyReport{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: suite.merchant.Id,
		Totals: &billingpb.RoyaltyReportTotals{
			TransactionsCount: 10,
			PayoutAmount:      500,
			VatAmount:         10,
			FeeAmount:         5,
		},
		Status:         billingpb.RoyaltyReportStatusPending,
		CreatedAt:      ptypes.TimestampNow(),
		PeriodFrom:     ptypes.TimestampNow(),
		PeriodTo:       ptypes.TimestampNow(),
		AcceptExpireAt: date,
		Currency:       suite.merchant.GetPayoutCurrency(),
	}
	err = suite.service.royaltyReportRepository.Insert(ctx, report, "", pkg.RoyaltyReportChangeSourceAuto)
	assert.NoError(suite.T(), err)

	req7 := &billingpb.ChangeRoyaltyReportRequest{
		ReportId:   report.Id,
		Status:     billingpb.RoyaltyReportStatusAccepted,
		Ip:         "127.0.0.1",
		MerchantId: suite.merchant.Id,
	}
	rsp7 := &billingpb.ResponseError{}
	err = suite.service.ChangeRoyaltyReport(context.TODO(), req7, rsp7)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp7.Status)
	assert.Empty(suite.T(), rsp7.Message)

	// control

	count = suite.mbRecordsCount(suite.merchant.Id, suite.merchant.GetPayoutCurrency())
	assert.EqualValues(suite.T(), count, 1)

	mbReq := &billingpb.GetMerchantBalanceRequest{MerchantId: suite.merchant.Id}
	mbRes := &billingpb.GetMerchantBalanceResponse{}
	err = suite.service.GetMerchantBalance(context.TODO(), mbReq, mbRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), mbRes.Status, billingpb.ResponseStatusOk)
	assert.Equal(suite.T(), mbRes.Item.MerchantId, suite.merchant.Id)
	assert.Equal(suite.T(), mbRes.Item.Currency, suite.merchant.GetPayoutCurrency())
	assert.Equal(suite.T(), mbRes.Item.Debit, float64(500))
	assert.Equal(suite.T(), mbRes.Item.Credit, float64(0))
	assert.Equal(suite.T(), mbRes.Item.RollingReserve, float64(0))
	assert.Equal(suite.T(), mbRes.Item.Total, float64(500))
}

func (suite *MerchantBalanceTestSuite) mbRecordsCount(merchantId, currency string) int64 {
	count, err := suite.service.merchantBalanceRepository.CountByIdAndCurrency(ctx, merchantId, currency)

	if err == nil {
		return count
	}

	if err == mongo.ErrNoDocuments {
		return 0
	}

	return -1
}
