package service

import (
	"context"
	"github.com/go-redis/redis"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang/protobuf/ptypes"
	"github.com/jinzhu/now"
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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
	"time"
)

type VatReportsTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger
	cache   database.CacheInterface

	projectFixedAmount *billingpb.Project
	paymentMethod      *billingpb.PaymentMethod
	paymentSystem      *billingpb.PaymentSystem

	logObserver *zap.Logger
	zapRecorder *observer.ObservedLogs
}

func Test_VatReports(t *testing.T) {
	suite.Run(t, new(VatReportsTestSuite))
}

func (suite *VatReportsTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}
	cfg.CardPayApiUrl = "https://sandbox.cardpay.com"
	cfg.OrderViewUpdateBatchSize = 20

	m, err := migrate.New(
		"file://../../migrations/tests",
		cfg.MongoDsn)
	assert.NoError(suite.T(), err, "Migrate init failed")

	err = m.Up()
	if err != nil && err.Error() != "no change" {
		suite.FailNow("Migrations failed", "%v", err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	opts := []mongodb.Option{mongodb.Context(ctx)}
	db, err := mongodb.NewDatabase(opts...)
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
		nil,
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	_, suite.projectFixedAmount, suite.paymentMethod, suite.paymentSystem = HelperCreateEntitiesForTests(suite.Suite, suite.service)

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)
}

func (suite *VatReportsTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *VatReportsTestSuite) TestVatReports_getLastVatReportTime() {
	_, _, err := suite.service.getLastVatReportTime(0)
	assert.Error(suite.T(), err)

	from, to, err := suite.service.getLastVatReportTime(int32(3))
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from, now.BeginningOfQuarter())
	assert.Equal(suite.T(), to, now.EndOfQuarter())

	from, to, err = suite.service.getLastVatReportTime(int32(1))
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from, now.BeginningOfMonth())
	assert.Equal(suite.T(), to, now.EndOfMonth())

	fromRef := now.BeginningOfMonth()
	toRef := now.EndOfMonth()

	if fromRef.Month()%2 == 0 {
		fromRef = now.BeginningOfMonth().AddDate(0, 0, -1)
		fromRef = now.New(fromRef).BeginningOfMonth()
	} else {
		toRef = now.EndOfMonth().AddDate(0, 0, 1)
		toRef = now.New(toRef).EndOfMonth()
	}

	from, to, err = suite.service.getLastVatReportTime(int32(2))
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from, fromRef)
	assert.Equal(suite.T(), to, toRef)
	assert.Equal(suite.T(), fromRef.Month()%2, time.Month(1))
	assert.Equal(suite.T(), toRef.Month()%2, time.Month(0))
}

func (suite *VatReportsTestSuite) TestVatReports_getVatReportTimeForDate() {

	t, err := time.Parse(time.RFC3339, "2019-06-29T11:45:26.371Z")
	assert.NoError(suite.T(), err)

	from, to, err := suite.service.getVatReportTimeForDate(int32(3), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-04-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-06-30T23:59:59Z")

	from, to, err = suite.service.getVatReportTimeForDate(int32(1), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-06-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-06-30T23:59:59Z")

	from, to, err = suite.service.getVatReportTimeForDate(int32(2), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-05-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-06-30T23:59:59Z")

	t, err = time.Parse(time.RFC3339, "2019-05-29T11:45:26.371Z")
	assert.NoError(suite.T(), err)
	from, to, err = suite.service.getVatReportTimeForDate(int32(2), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-05-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-06-30T23:59:59Z")

	t, err = time.Parse(time.RFC3339, "2019-07-29T11:45:26.371Z")
	assert.NoError(suite.T(), err)
	from, to, err = suite.service.getVatReportTimeForDate(int32(2), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-07-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-08-31T23:59:59Z")

	t, err = time.Parse(time.RFC3339, "2019-08-29T11:45:26.371Z")
	assert.NoError(suite.T(), err)
	from, to, err = suite.service.getVatReportTimeForDate(int32(2), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-07-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-08-31T23:59:59Z")

	t, err = time.Parse(time.RFC3339, "2019-04-01T00:00:00Z")
	assert.NoError(suite.T(), err)
	from, to, err = suite.service.getVatReportTimeForDate(int32(3), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-04-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-06-30T23:59:59Z")

	t, err = time.Parse(time.RFC3339, "2019-06-30T23:59:59Z")
	assert.NoError(suite.T(), err)
	from, to, err = suite.service.getVatReportTimeForDate(int32(3), t)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), from.Format(time.RFC3339), "2019-04-01T00:00:00Z")
	assert.Equal(suite.T(), to.Format(time.RFC3339), "2019-06-30T23:59:59Z")
}

func (suite *VatReportsTestSuite) TestVatReports_ProcessVatReports() {
	amounts := []float64{100, 10}
	currencies := []string{"RUB", "USD"}
	countries := []string{"RU", "FI"}
	var orders []*billingpb.Order
	numberOfOrders := 30

	suite.projectFixedAmount.Status = billingpb.ProjectStatusInProduction
	if err := suite.service.project.Update(context.TODO(), suite.projectFixedAmount); err != nil {
		suite.FailNow("Update project test data failed", "%v", err)
	}

	count := 0
	for count < numberOfOrders {
		order := HelperCreateAndPayOrder(
			suite.Suite,
			suite.service,
			amounts[count%2],
			currencies[count%2],
			countries[count%2],
			suite.projectFixedAmount,
			suite.paymentMethod,
		)
		assert.NotNil(suite.T(), order)
		orders = append(orders, order)

		count++
	}

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(context.TODO(), suite.paymentSystem)
	assert.NoError(suite.T(), err)

	for i, order := range orders {
		if i%3 == 0 {
			continue
		}
		refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, false)
		assert.NotNil(suite.T(), refund)
	}

	req := &billingpb.ProcessVatReportsRequest{
		Date: ptypes.TimestampNow(),
	}
	err = suite.service.ProcessVatReports(context.TODO(), req, &billingpb.EmptyResponse{})
	assert.NoError(suite.T(), err)

	repRes := billingpb.VatReportsResponse{}

	err = suite.service.GetVatReportsForCountry(context.TODO(), &billingpb.VatReportsRequest{Country: "RU"}, &repRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), repRes.Status, billingpb.ResponseStatusOk)
	assert.NotNil(suite.T(), repRes.Data)
	assert.Equal(suite.T(), repRes.Data.Count, int32(1))

	report := repRes.Data.Items[0]
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), "RU", report.Country)
	assert.Equal(suite.T(), "RUB", report.Currency)
	assert.EqualValues(suite.T(), 25, report.TransactionsCount)
	assert.EqualValues(suite.T(), 600, report.GrossRevenue)
	assert.EqualValues(suite.T(), 100, report.VatAmount)
	assert.EqualValues(suite.T(), 144.36, report.FeesAmount)
	assert.EqualValues(suite.T(), 0, report.DeductionAmount)
	assert.EqualValues(suite.T(), 600, report.CountryAnnualTurnover)
	assert.EqualValues(suite.T(), 4393.9, report.WorldAnnualTurnover)
	assert.Equal(suite.T(), pkg.VatReportStatusThreshold, report.Status)

	err = suite.service.GetVatReportsForCountry(context.TODO(), &billingpb.VatReportsRequest{Country: "FI"}, &repRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), repRes.Status, billingpb.ResponseStatusOk)
	assert.NotNil(suite.T(), repRes.Data)
	assert.Equal(suite.T(), repRes.Data.Count, int32(1))

	report = repRes.Data.Items[0]
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), "FI", report.Country)
	assert.Equal(suite.T(), "EUR", report.Currency)
	assert.EqualValues(suite.T(), 25, report.TransactionsCount)
	assert.EqualValues(suite.T(), 54.2, report.GrossRevenue)
	assert.EqualValues(suite.T(), 9.03, report.VatAmount)
	assert.EqualValues(suite.T(), 8.89, report.FeesAmount)
	assert.EqualValues(suite.T(), 0, report.DeductionAmount)
	assert.EqualValues(suite.T(), 54, report.CountryAnnualTurnover)
	assert.EqualValues(suite.T(), 62.77, report.WorldAnnualTurnover)
	assert.Equal(suite.T(), pkg.VatReportStatusThreshold, report.Status)

	assert.NoError(suite.T(), err)
}

func (suite *VatReportsTestSuite) TestVatReports_PaymentDateSet() {
	zap.ReplaceGlobals(suite.logObserver)
	suite.service.centrifugoDashboard = newCentrifugo(suite.service.cfg.CentrifugoDashboard, mocks.NewClientStatusOk())

	nowTimestamp := time.Now().Unix()

	vatReport := &billingpb.VatReport{
		Id:                    primitive.NewObjectID().Hex(),
		Country:               "RU",
		VatRate:               20,
		Currency:              "RUB",
		Status:                pkg.VatReportStatusNeedToPay,
		TransactionsCount:     999,
		GrossRevenue:          100500,
		VatAmount:             100500,
		FeesAmount:            0,
		DeductionAmount:       0,
		CountryAnnualTurnover: 100500,
		WorldAnnualTurnover:   100500,
		CorrectionAmount:      0,
		AmountsApproximate:    false,
		DateFrom:              ptypes.TimestampNow(),
		DateTo:                ptypes.TimestampNow(),
		CreatedAt:             ptypes.TimestampNow(),
		UpdatedAt:             ptypes.TimestampNow(),
		PayUntilDate:          ptypes.TimestampNow(),
	}

	err := suite.service.vatReportRepository.Insert(context.TODO(), vatReport)
	assert.NoError(suite.T(), err)

	vr, err := suite.service.vatReportRepository.GetById(context.TODO(), vatReport.Id)
	assert.NoError(suite.T(), err)
	assert.NotEqual(suite.T(), vr.Status, pkg.VatReportStatusPaid)
	assert.EqualValues(suite.T(), -62135596800, vr.PaidAt.Seconds)

	req := &billingpb.UpdateVatReportStatusRequest{
		Id:     vr.Id,
		Status: pkg.VatReportStatusPaid,
	}
	res := &billingpb.ResponseError{}
	err = suite.service.UpdateVatReportStatus(context.TODO(), req, res)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Status, billingpb.ResponseStatusOk)
	assert.Empty(suite.T(), res.Message)

	vr, err = suite.service.vatReportRepository.GetById(context.TODO(), vatReport.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), vr.Status, pkg.VatReportStatusPaid)
	assert.GreaterOrEqual(suite.T(), nowTimestamp, vr.PaidAt.Seconds)

	messages := suite.zapRecorder.All()
	assert.Regexp(suite.T(), "dashboard", messages[0].Message)
}

func (suite *VatReportsTestSuite) TestVatReports_ProcessVatReports_OnlyTestOrders() {
	amounts := []float64{100, 10}
	currencies := []string{"RUB", "USD"}
	countries := []string{"RU", "FI"}
	var orders []*billingpb.Order
	numberOfOrders := 30

	assert.False(suite.T(), suite.projectFixedAmount.IsProduction())

	count := 0
	for count < numberOfOrders {
		order := HelperCreateAndPayOrder(
			suite.Suite,
			suite.service,
			amounts[count%2],
			currencies[count%2],
			countries[count%2],
			suite.projectFixedAmount,
			suite.paymentMethod,
		)
		assert.NotNil(suite.T(), order)
		orders = append(orders, order)

		count++
	}

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(context.TODO(), suite.paymentSystem)
	assert.NoError(suite.T(), err)

	for _, order := range orders {
		refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount*0.5, false)
		assert.NotNil(suite.T(), refund)
	}

	req := &billingpb.ProcessVatReportsRequest{
		Date: ptypes.TimestampNow(),
	}
	err = suite.service.ProcessVatReports(context.TODO(), req, &billingpb.EmptyResponse{})
	assert.NoError(suite.T(), err)

	repRes := billingpb.VatReportsResponse{}

	err = suite.service.GetVatReportsForCountry(context.TODO(), &billingpb.VatReportsRequest{Country: "RU"}, &repRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), repRes.Status, billingpb.ResponseStatusOk)
	assert.NotNil(suite.T(), repRes.Data)
	assert.Equal(suite.T(), repRes.Data.Count, int32(1))

	report := repRes.Data.Items[0]
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), report.Country, "RU")
	assert.Equal(suite.T(), report.Currency, "RUB")
	assert.EqualValues(suite.T(), report.TransactionsCount, 0)
	assert.EqualValues(suite.T(), report.GrossRevenue, 0)
	assert.EqualValues(suite.T(), report.VatAmount, 0)
	assert.EqualValues(suite.T(), report.FeesAmount, 0)
	assert.EqualValues(suite.T(), report.DeductionAmount, 0)
	assert.EqualValues(suite.T(), report.CountryAnnualTurnover, 0)
	assert.EqualValues(suite.T(), report.WorldAnnualTurnover, 0)
	assert.Equal(suite.T(), report.Status, pkg.VatReportStatusThreshold)

	err = suite.service.GetVatReportsForCountry(context.TODO(), &billingpb.VatReportsRequest{Country: "FI"}, &repRes)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), repRes.Status, billingpb.ResponseStatusOk)
	assert.NotNil(suite.T(), repRes.Data)
	assert.Equal(suite.T(), repRes.Data.Count, int32(1))

	report = repRes.Data.Items[0]
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), report.Country, "FI")
	assert.Equal(suite.T(), report.Currency, "EUR")
	assert.EqualValues(suite.T(), report.TransactionsCount, 0)
	assert.EqualValues(suite.T(), report.GrossRevenue, 0)
	assert.EqualValues(suite.T(), report.VatAmount, 0)
	assert.EqualValues(suite.T(), report.FeesAmount, 0)
	assert.EqualValues(suite.T(), report.DeductionAmount, 0)
	assert.EqualValues(suite.T(), report.CountryAnnualTurnover, 0)
	assert.EqualValues(suite.T(), report.WorldAnnualTurnover, 0)
	assert.Equal(suite.T(), report.Status, pkg.VatReportStatusThreshold)

	assert.NoError(suite.T(), err)
}
