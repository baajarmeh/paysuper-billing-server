package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/internal/payment_system"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	"github.com/paysuper/paysuper-proto/go/postmarkpb"
	"github.com/paysuper/paysuper-proto/go/reporterpb"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"net/http"
	"testing"
	"time"
)

type RoyaltyReportTestSuite struct {
	suite.Suite
	service    *Service
	log        *zap.Logger
	cache      database.CacheInterface
	httpClient *http.Client

	project       *billingpb.Project
	project1      *billingpb.Project
	project2      *billingpb.Project
	project3      *billingpb.Project
	merchant      *billingpb.Merchant
	merchant1     *billingpb.Merchant
	merchant2     *billingpb.Merchant
	merchant3     *billingpb.Merchant
	paymentMethod *billingpb.PaymentMethod
	paymentSystem *billingpb.PaymentSystem
	customer      *billingpb.Customer

	logObserver *zap.Logger
	zapRecorder *observer.ObservedLogs
}

func Test_RoyaltyReport(t *testing.T) {
	suite.Run(t, new(RoyaltyReportTestSuite))
}

func (suite *RoyaltyReportTestSuite) SetupTest() {
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

	reporterMock := &reportingMocks.ReporterService{}
	reporterMock.On("CreateFile", mock2.Anything, mock2.Anything, mock2.Anything).
		Return(&reporterpb.CreateFileResponse{Status: billingpb.ResponseStatusOk}, nil)

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
		reporterMock,
		mocks.NewFormatterOK(),
		mocks.NewBrokerMockOk(),
		&casbinMocks.CasbinService{},
		mocks.NewNotifierOk(),
		mocks.NewBrokerMockOk(),
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.merchant, suite.project, suite.paymentMethod, suite.paymentSystem, suite.customer = HelperCreateEntitiesForTests(suite.Suite, suite.service)

	suite.project.Status = billingpb.ProjectStatusInProduction
	if err := suite.service.project.Update(context.TODO(), suite.project); err != nil {
		suite.FailNow("Update project test data failed", "%v", err)
	}

	suite.merchant1 = HelperCreateMerchant(suite.Suite, suite.service, "USD", "RU", suite.paymentMethod, 0, suite.merchant.OperatingCompanyId)
	suite.merchant2 = HelperCreateMerchant(suite.Suite, suite.service, "USD", "RU", suite.paymentMethod, 0, suite.merchant.OperatingCompanyId)
	suite.merchant3 = HelperCreateMerchant(suite.Suite, suite.service, "USD", "RU", suite.paymentMethod, 0, suite.merchant.OperatingCompanyId)

	suite.project1 = &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         "default",
		LimitsCurrency:           "USD",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       false,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   billingpb.ProjectStatusInProduction,
		MerchantId:               suite.merchant1.Id,
		VatPayer:                 billingpb.VatPayerBuyer,
	}
	suite.project2 = &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         "default",
		LimitsCurrency:           "USD",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 2"},
		IsProductsCheckout:       false,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 2 secret key",
		Status:                   billingpb.ProjectStatusInProduction,
		MerchantId:               suite.merchant2.Id,
		VatPayer:                 billingpb.VatPayerBuyer,
	}

	suite.project3 = &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         "default",
		LimitsCurrency:           "USD",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 2"},
		IsProductsCheckout:       false,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 2 secret key",
		Status:                   billingpb.ProjectStatusDraft,
		MerchantId:               suite.merchant3.Id,
		VatPayer:                 billingpb.VatPayerBuyer,
	}

	projects := []*billingpb.Project{suite.project1, suite.project2, suite.project3}
	err = suite.service.project.MultipleInsert(context.TODO(), projects)

	if err != nil {
		suite.FailNow("Insert projects test data failed", "%v", err)
	}

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)
}

func (suite *RoyaltyReportTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_CreateRoyaltyReport_AllMerchants_Ok() {
	reporterMock := &reportingMocks.ReporterService{}
	reporterMock.On("CreateFile", mock2.Anything, mock2.Anything, mock2.Anything).
		Return(&reporterpb.CreateFileResponse{Status: billingpb.ResponseStatusOk}, nil).
		Run(func(args mock2.Arguments) {
			incomingCtx := args.Get(0).(context.Context)
			incomingReq := args.Get(1).(*reporterpb.ReportFile)
			var params map[string]interface{}

			if incomingReq.Params != nil {
				if err := json.Unmarshal(incomingReq.Params, &params); err != nil {
					return
				}
			}
			// we must take real RoyaltyReportId value from request,
			// to awoid royaltyReportErrorReportNotFound during the RoyaltyReportPdfUploaded process
			req := &billingpb.RoyaltyReportPdfUploadedRequest{
				Id:              primitive.NewObjectID().Hex(),
				RoyaltyReportId: fmt.Sprintf("%s", params[reporterpb.ParamsFieldId]),
				Filename:        "somename.pdf",
				RetentionTime:   int32(123),
				Content:         []byte{},
			}

			res := &billingpb.RoyaltyReportPdfUploadedResponse{}
			_ = suite.service.RoyaltyReportPdfUploaded(incomingCtx, req, res)
		})
	suite.service.reporterService = reporterMock

	projects := []*billingpb.Project{suite.project, suite.project1, suite.project2}
	orderIds := make([]primitive.ObjectID, 0)

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			order := suite.createOrder(v)
			oid, err := primitive.ObjectIDFromHex(order.Id)
			assert.NoError(suite.T(), err)
			orderIds = append(orderIds, oid)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	postmarkBrokerMock := &mocks.BrokerInterface{}
	postmarkBrokerMock.On("Publish", postmarkpb.PostmarkSenderTopicName, mock.Anything, mock.Anything).Return(nil, nil)

	// Warning! For correct counting of calls for sending royalty report email,
	// replacing of postmarkBroker with custom mock must be here
	// to prevent counting a calls for sending transaction success mails due to orders creation and payment
	suite.service.postmarkBroker = postmarkBrokerMock

	orderViews := HelperGetOrdersViewsMap(suite.Suite, suite.service, orderIds)
	for _, v := range orderViews {
		assert.Contains(suite.T(), v, "royalty_report_id")
		assert.Empty(suite.T(), v["royalty_report_id"])
	}

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	orderViews = HelperGetOrdersViewsMap(suite.Suite, suite.service, orderIds)
	for _, v := range orderViews {
		assert.Contains(suite.T(), v, "royalty_report_id")
		assert.NotEmpty(suite.T(), v["royalty_report_id"])
	}

	err = suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)
	err = suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)
	err = suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	orderViewsByReports := make(map[string]int)
	orderViews = HelperGetOrdersViewsMap(suite.Suite, suite.service, orderIds)
	for _, v := range orderViews {
		assert.Contains(suite.T(), v, "royalty_report_id")
		assert.NotEmpty(suite.T(), v["royalty_report_id"])

		orderViewsByReports[v["royalty_report_id"].(string)]++
	}

	assert.Contains(suite.T(), rsp.Merchants, suite.merchant.Id)
	assert.Contains(suite.T(), rsp.Merchants, suite.merchant1.Id)
	assert.Contains(suite.T(), rsp.Merchants, suite.merchant2.Id)

	loc, err := time.LoadLocation("Europe/Moscow")
	assert.NoError(suite.T(), err)

	to := now.Monday().Add(-1 * time.Millisecond).In(loc)

	from := to.Add(-time.Duration(suite.service.cfg.RoyaltyReportPeriod) * time.Second).Add(2 * time.Millisecond).In(loc)
	from = now.New(from).BeginningOfDay()

	reports, err := suite.service.royaltyReportRepository.GetByPeriod(context.TODO(), from, to)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), reports)
	assert.Len(suite.T(), reports, 3)

	var existMerchants []string

	for _, v := range reports {
		assert.NotZero(suite.T(), v.Id)
		assert.NotZero(suite.T(), v.Totals)
		assert.NotZero(suite.T(), v.Status)
		assert.NotZero(suite.T(), v.MerchantId)
		assert.NotZero(suite.T(), v.PeriodFrom)
		assert.NotZero(suite.T(), v.PeriodTo)
		assert.NotZero(suite.T(), v.AcceptExpireAt)
		assert.NotZero(suite.T(), v.Totals.PayoutAmount)
		assert.NotZero(suite.T(), v.Totals.VatAmount)
		assert.NotZero(suite.T(), v.Totals.FeeAmount)
		assert.NotZero(suite.T(), v.Totals.TransactionsCount)

		t, err := ptypes.Timestamp(v.PeriodFrom)
		assert.NoError(suite.T(), err)
		t1, err := ptypes.Timestamp(v.PeriodTo)
		assert.NoError(suite.T(), err)

		assert.Equal(suite.T(), t.In(loc), from)
		assert.Equal(suite.T(), t1.In(loc), to)
		assert.InDelta(suite.T(), suite.service.cfg.RoyaltyReportAcceptTimeout, v.AcceptExpireAt.Seconds-time.Now().Unix(), 10)

		existMerchants = append(existMerchants, v.MerchantId)

		assert.EqualValues(suite.T(), orderViewsByReports[v.Id], v.Totals.TransactionsCount)
	}

	assert.Contains(suite.T(), existMerchants, suite.merchant.Id)
	assert.Contains(suite.T(), existMerchants, suite.merchant1.Id)
	assert.Contains(suite.T(), existMerchants, suite.merchant2.Id)

	// check for sending requests for pdf generation
	reporterMock.AssertNumberOfCalls(suite.T(), "CreateFile", len(reports))
	// check for requests to send emails with generated pdfs
	postmarkBrokerMock.AssertNumberOfCalls(suite.T(), "Publish", len(reports))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_CreateRoyaltyReport_SelectedMerchants_Ok() {
	projects := []*billingpb.Project{suite.project, suite.project1}

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{
		Merchants: []string{suite.project.GetMerchantId(), suite.project1.GetMerchantId()},
	}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), reports)
	assert.Len(suite.T(), reports, len(req.Merchants))

	var existMerchants []string

	loc, err := time.LoadLocation("Europe/Moscow")
	assert.NoError(suite.T(), err)

	to := now.Monday().Add(-1 * time.Millisecond).In(loc)

	from := to.Add(-time.Duration(suite.service.cfg.RoyaltyReportPeriod) * time.Second).Add(2 * time.Millisecond).In(loc)
	from = now.New(from).BeginningOfDay()

	for _, v := range reports {
		assert.NotZero(suite.T(), v.Id)
		assert.NotZero(suite.T(), v.Totals)
		assert.NotZero(suite.T(), v.Status)
		assert.NotZero(suite.T(), v.MerchantId)
		assert.NotZero(suite.T(), v.PeriodFrom)
		assert.NotZero(suite.T(), v.PeriodTo)
		assert.NotZero(suite.T(), v.AcceptExpireAt)
		assert.NotZero(suite.T(), v.Totals.PayoutAmount)
		assert.NotZero(suite.T(), v.Currency)
		assert.NotZero(suite.T(), v.Totals.VatAmount)
		assert.NotZero(suite.T(), v.Totals.FeeAmount)
		assert.NotZero(suite.T(), v.Totals.TransactionsCount)

		t, err := ptypes.Timestamp(v.PeriodFrom)
		assert.NoError(suite.T(), err)
		t1, err := ptypes.Timestamp(v.PeriodTo)
		assert.NoError(suite.T(), err)

		assert.Equal(suite.T(), t.In(loc), from)
		assert.Equal(suite.T(), t1.In(loc), to)
		assert.InDelta(suite.T(), suite.service.cfg.RoyaltyReportAcceptTimeout, v.AcceptExpireAt.Seconds-time.Now().Unix(), 10)

		existMerchants = append(existMerchants, v.MerchantId)
	}

	assert.Contains(suite.T(), existMerchants, suite.merchant.Id)
	assert.Contains(suite.T(), existMerchants, suite.merchant1.Id)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_CreateRoyaltyReport_EmptyMerchants_Error() {
	req := &billingpb.CreateRoyaltyReportRequest{
		Merchants: []string{"incorrect_hex"},
	}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err := suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), reports)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_CreateRoyaltyReport_NotExistMerchant_Error() {
	req := &billingpb.CreateRoyaltyReportRequest{
		Merchants: []string{primitive.NewObjectID().Hex()},
	}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err := suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), rsp.Merchants)
	assert.Len(suite.T(), rsp.Merchants, 0)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), reports)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReports_Ok() {
	projects := []*billingpb.Project{suite.project, suite.project1, suite.project2}

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	to := now.Monday().Add(-1 * time.Nanosecond)

	from := to.Add(-time.Duration(suite.service.cfg.RoyaltyReportPeriod) * time.Second).Add(1 * time.Nanosecond)
	from = now.New(from).BeginningOfDay()

	oid, _ := primitive.ObjectIDFromHex(suite.project.GetMerchantId())
	query := bson.M{"merchant_id": oid}
	set := bson.M{"$set": bson.M{"period_from": from, "period_to": to}}
	err = suite.service.royaltyReportRepository.UpdateMany(context.TODO(), query, set)
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	req1 := &billingpb.ListRoyaltyReportsRequest{MerchantId: suite.project.GetMerchantId()}
	rsp1 := &billingpb.ListRoyaltyReportsResponse{}
	err = suite.service.ListRoyaltyReports(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), int64(1), rsp1.Data.Count)
	assert.Len(suite.T(), rsp1.Data.Items, int(rsp1.Data.Count))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReports_FindById_Ok() {
	projects := []*billingpb.Project{suite.project, suite.project1, suite.project2}

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)

	req1 := &billingpb.ListRoyaltyReportsRequest{MerchantId: reports[0].MerchantId}
	rsp1 := &billingpb.ListRoyaltyReportsResponse{}
	err = suite.service.ListRoyaltyReports(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 1, rsp1.Data.Count)
	assert.Len(suite.T(), rsp1.Data.Items, int(rsp1.Data.Count))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReports_Merchant_NotFound() {
	req := &billingpb.ListRoyaltyReportsRequest{MerchantId: primitive.NewObjectID().Hex()}
	rsp := &billingpb.ListRoyaltyReportsResponse{}
	err := suite.service.ListRoyaltyReports(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 0, rsp.Data.Count)
	assert.Len(suite.T(), rsp.Data.Items, int(rsp.Data.Count))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReports_FindByMerchantId_Ok() {
	projects := []*billingpb.Project{suite.project, suite.project1, suite.project2}

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	to := now.Monday().Add(-1 * time.Nanosecond)

	from := to.Add(-time.Duration(suite.service.cfg.RoyaltyReportPeriod) * time.Second).Add(1 * time.Nanosecond)
	from = now.New(from).BeginningOfDay()

	oid, _ := primitive.ObjectIDFromHex(suite.project.GetMerchantId())
	query := bson.M{"merchant_id": oid}
	set := bson.M{"$set": bson.M{"period_from": from, "period_to": to}}
	err = suite.service.royaltyReportRepository.UpdateMany(context.TODO(), query, set)
	assert.NoError(suite.T(), err)

	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	req1 := &billingpb.ListRoyaltyReportsRequest{MerchantId: suite.project.GetMerchantId()}
	rsp1 := &billingpb.ListRoyaltyReportsResponse{}
	err = suite.service.ListRoyaltyReports(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 1, rsp1.Data.Count)
	assert.Len(suite.T(), rsp1.Data.Items, int(rsp1.Data.Count))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReports_FindByMerchantId_NotFound() {
	req := &billingpb.ListRoyaltyReportsRequest{MerchantId: primitive.NewObjectID().Hex()}
	rsp := &billingpb.ListRoyaltyReportsResponse{}
	err := suite.service.ListRoyaltyReports(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 0, rsp.Data.Count)
	assert.Empty(suite.T(), rsp.Data.Items)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReports_FindByPeriod_Ok() {
	projects := []*billingpb.Project{suite.project, suite.project1, suite.project2}

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	oid, _ := primitive.ObjectIDFromHex(suite.project.GetMerchantId())
	query := bson.M{"merchant_id": oid}
	set := bson.M{"$set": bson.M{"created_at": time.Now().Add(24 * -time.Hour)}}
	err = suite.service.royaltyReportRepository.UpdateMany(context.TODO(), query, set)
	assert.NoError(suite.T(), err)

	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	req1 := &billingpb.ListRoyaltyReportsRequest{
		MerchantId: suite.project.GetMerchantId(),
		PeriodFrom: time.Now().Add(30 * -time.Hour).Format(billingpb.FilterDatetimeFormat),
		PeriodTo:   time.Now().Add(20 * -time.Hour).Format(billingpb.FilterDatetimeFormat),
	}
	rsp1 := &billingpb.ListRoyaltyReportsResponse{}
	err = suite.service.ListRoyaltyReports(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), int64(1), rsp1.Data.Count)
	assert.Len(suite.T(), rsp1.Data.Items, int(rsp1.Data.Count))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReports_FindByPeriod_NotFound() {
	req := &billingpb.ListRoyaltyReportsRequest{
		MerchantId: suite.project.GetMerchantId(),
		PeriodFrom: time.Now().Format(billingpb.FilterDatetimeFormat),
		PeriodTo:   time.Now().Format(billingpb.FilterDatetimeFormat),
	}
	rsp := &billingpb.ListRoyaltyReportsResponse{}
	err := suite.service.ListRoyaltyReports(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 0, rsp.Data.Count)
	assert.Empty(suite.T(), rsp.Data.Items)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ChangeRoyaltyReport_Ok() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)

	zap.ReplaceGlobals(suite.logObserver)
	suite.service.centrifugoDashboard = newCentrifugo(suite.service.cfg.CentrifugoDashboard, payment_system.NewClientStatusOk())

	req1 := &billingpb.ChangeRoyaltyReportRequest{
		ReportId:   reports[0].Id,
		MerchantId: reports[0].MerchantId,
		Status:     billingpb.RoyaltyReportStatusAccepted,
		Ip:         "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.ChangeRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)

	messages := suite.zapRecorder.All()
	assert.Regexp(suite.T(), "dashboard", messages[0].Message)

	report, err := suite.service.royaltyReportRepository.GetById(context.TODO(), reports[0].Id)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusAccepted, report.Status)
	assert.False(suite.T(), report.IsAutoAccepted)

	changes, err := suite.service.royaltyReportRepository.GetRoyaltyHistoryById(context.TODO(), report.Id)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), changes, 2)
	assert.Equal(suite.T(), req1.Ip, changes[1].Ip)
	assert.Equal(suite.T(), pkg.RoyaltyReportChangeSourceAdmin, changes[1].Source)

	centrifugoCl, ok := suite.httpClient.Transport.(*payment_system.TransportStatusOk)
	assert.True(suite.T(), ok)
	assert.NoError(suite.T(), centrifugoCl.Err)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ChangeRoyaltyReport_DisputeAndCorrection_Ok() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)
	assert.EqualValues(suite.T(), -62135596800, reports[0].AcceptedAt.Seconds)
	assert.Len(suite.T(), reports[0].Summary.Corrections, 0)
	assert.Equal(suite.T(), reports[0].Totals.CorrectionAmount, float64(0))

	req1 := &billingpb.MerchantReviewRoyaltyReportRequest{
		ReportId:      reports[0].Id,
		IsAccepted:    false,
		DisputeReason: "unit-test",
		Ip:            "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.MerchantReviewRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)

	report, err := suite.service.royaltyReportRepository.GetById(context.TODO(), reports[0].Id)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusDispute, report.Status)

	req2 := &billingpb.ChangeRoyaltyReportRequest{
		ReportId:   report.Id,
		Status:     billingpb.RoyaltyReportStatusPending,
		MerchantId: report.MerchantId,
		Correction: &billingpb.ChangeRoyaltyReportCorrection{
			Amount: 10,
			Reason: "unit-test",
		},
		Ip: "127.0.0.1",
	}
	rsp2 := &billingpb.ResponseError{}
	err = suite.service.ChangeRoyaltyReport(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	report, err = suite.service.royaltyReportRepository.GetById(context.TODO(), report.Id)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, report.Status)
	assert.Len(suite.T(), report.Summary.Corrections, 1)
	assert.Equal(suite.T(), report.Totals.CorrectionAmount, float64(10))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_MerchantReviewRoyaltyReport_Accepted_Ok() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)
	assert.EqualValues(suite.T(), -62135596800, reports[0].AcceptedAt.Seconds)

	reports[0].Status = billingpb.RoyaltyReportStatusPending
	err = suite.service.royaltyReportRepository.Update(context.TODO(), reports[0], "127.0.0.1", pkg.RoyaltyReportChangeSourceMerchant)
	assert.NoError(suite.T(), err)

	req1 := &billingpb.MerchantReviewRoyaltyReportRequest{
		ReportId:   reports[0].Id,
		IsAccepted: true,
		Ip:         "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.MerchantReviewRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)

	report, err := suite.service.royaltyReportRepository.GetById(context.TODO(), reports[0].Id)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusAccepted, report.Status)
	assert.NotEqual(suite.T(), int64(-62135596800), report.AcceptedAt.Seconds)

	changes, err := suite.service.royaltyReportRepository.GetRoyaltyHistoryById(context.TODO(), reports[0].Id)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), changes, 3)
	assert.Equal(suite.T(), req1.Ip, changes[1].Ip)
	assert.Equal(suite.T(), pkg.RoyaltyReportChangeSourceMerchant, changes[1].Source)

	centrifugoCl, ok := suite.httpClient.Transport.(*payment_system.TransportStatusOk)
	assert.True(suite.T(), ok)
	assert.NoError(suite.T(), centrifugoCl.Err)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_MerchantReviewRoyaltyReport_Dispute_Ok() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)
	assert.EqualValues(suite.T(), -62135596800, reports[0].AcceptedAt.Seconds)

	req1 := &billingpb.MerchantReviewRoyaltyReportRequest{
		ReportId:      reports[0].Id,
		IsAccepted:    false,
		DisputeReason: "unit-test",
		Ip:            "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.MerchantReviewRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)

	report, err := suite.service.royaltyReportRepository.GetById(context.TODO(), reports[0].Id)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusDispute, report.Status)

	changes, err := suite.service.royaltyReportRepository.GetRoyaltyHistoryById(context.TODO(), reports[0].Id)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), changes, 2)
	assert.Equal(suite.T(), req1.Ip, changes[1].Ip)
	assert.Equal(suite.T(), pkg.RoyaltyReportChangeSourceMerchant, changes[1].Source)

	centrifugoCl, ok := suite.httpClient.Transport.(*payment_system.TransportStatusOk)
	assert.True(suite.T(), ok)
	assert.NoError(suite.T(), centrifugoCl.Err)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ChangeRoyaltyReport_ReportNotFound_Error() {
	req := &billingpb.ChangeRoyaltyReportRequest{
		ReportId: primitive.NewObjectID().Hex(),
		Status:   billingpb.RoyaltyReportStatusPending,
		Ip:       "127.0.0.1",
	}
	rsp := &billingpb.ResponseError{}
	err := suite.service.ChangeRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), royaltyReportErrorReportNotFound, rsp.Message)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ChangeRoyaltyReport_ChangeNotAllowed_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)

	req1 := &billingpb.ChangeRoyaltyReportRequest{
		ReportId:   reports[0].Id,
		Status:     billingpb.RoyaltyReportStatusCanceled,
		MerchantId: reports[0].MerchantId,
		Ip:         "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.ChangeRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), royaltyReportErrorReportStatusChangeDenied, rsp1.Message)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ChangeRoyaltyReport_StatusPaymentError_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)

	reports[0].Status = billingpb.RoyaltyReportStatusPending
	err = suite.service.royaltyReportRepository.Update(context.TODO(), reports[0], "", pkg.RoyaltyReportChangeSourceAuto)
	assert.NoError(suite.T(), err)

	req1 := &billingpb.ChangeRoyaltyReportRequest{
		ReportId:   reports[0].Id,
		Status:     billingpb.RoyaltyReportStatusDispute,
		MerchantId: reports[0].MerchantId,
		Ip:         "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.ChangeRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReportOrders_Ok() {
	for i := 0; i < 5; i++ {
		suite.createOrder(suite.project)
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)

	req1 := &billingpb.ListRoyaltyReportOrdersRequest{ReportId: reports[0].Id, Limit: 5, Offset: 0}
	rsp1 := &billingpb.TransactionsResponse{}
	err = suite.service.ListRoyaltyReportOrders(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.NotEmpty(suite.T(), rsp1.Data)
	assert.EqualValues(suite.T(), rsp1.Data.Count, req1.Limit)
	assert.Len(suite.T(), rsp1.Data.Items, int(req1.Limit))

	for _, v := range rsp1.Data.Items {
		assert.NotZero(suite.T(), v.CreatedAt)
		assert.NotZero(suite.T(), v.CountryCode)
		assert.NotZero(suite.T(), v.Transaction)
		assert.NotZero(suite.T(), v.PaymentMethod)
		assert.NotZero(suite.T(), v.TotalPaymentAmount)
		assert.NotZero(suite.T(), v.Currency)
	}
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReportOrders_ReportNotFound_Error() {
	req := &billingpb.ListRoyaltyReportOrdersRequest{ReportId: primitive.NewObjectID().Hex(), Limit: 5, Offset: 0}
	rsp := &billingpb.TransactionsResponse{}
	err := suite.service.ListRoyaltyReportOrders(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ListRoyaltyReportOrders_OrdersNotFound_Error() {
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), reports)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_SendRoyaltyReportNotification_MerchantNotFound_Error() {
	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)

	report := &billingpb.RoyaltyReport{
		MerchantId: primitive.NewObjectID().Hex(),
	}
	suite.service.sendRoyaltyReportNotification(context.Background(), report)
	assert.True(suite.T(), recorded.Len() == 2)

	messages := recorded.All()
	assert.Contains(suite.T(), messages[1].Message, "Merchant not found")
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_SendRoyaltyReportNotification_CentrifugoSendError() {
	for i := 0; i < 5; i++ {
		suite.createOrder(suite.project)
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)

	ci := &mocks.CentrifugoInterface{}
	ci.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("error"))
	suite.service.centrifugoDashboard = ci

	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)

	suite.service.sendRoyaltyReportNotification(context.Background(), reports[0])
	assert.True(suite.T(), recorded.Len() == 1)

	messages := recorded.All()
	assert.Contains(suite.T(), messages[0].Message, "[Centrifugo] Send merchant notification about new royalty report failed")
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_AutoAcceptRoyaltyReports_Ok() {
	projects := []*billingpb.Project{suite.project, suite.project1, suite.project2}

	ci := &mocks.CentrifugoInterface{}
	ci.On("Publish", mock2.Anything, mock2.Anything, mock2.Anything).Return(nil)
	suite.service.centrifugoDashboard = ci

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	oid, err := primitive.ObjectIDFromHex(suite.project.GetMerchantId())
	assert.NoError(suite.T(), err)

	err = suite.service.royaltyReportRepository.UpdateMany(context.TODO(),
		bson.M{"merchant_id": oid},
		bson.M{
			"$set": bson.M{
				"accept_expire_at": time.Now().Add(-time.Duration(336) * time.Hour),
				"status":           billingpb.RoyaltyReportStatusPending,
			},
		},
	)
	assert.NoError(suite.T(), err)

	req1 := &billingpb.EmptyRequest{}
	rsp1 := &billingpb.EmptyResponse{}
	err = suite.service.AutoAcceptRoyaltyReports(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)

	for _, v := range reports {
		if v.MerchantId == suite.project.GetMerchantId() {
			assert.Equal(suite.T(), billingpb.RoyaltyReportStatusAccepted, v.Status)
			assert.True(suite.T(), v.IsAutoAccepted)
		} else {
			assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, v.Status)
			assert.False(suite.T(), v.IsAutoAccepted)
		}
	}
}

func (suite *RoyaltyReportTestSuite) createOrder(project *billingpb.Project) *billingpb.Order {
	customerRequest := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Email: &billingpb.TokenUserEmailValue{
				Value: "test@unit.test",
			},
			Phone: &billingpb.TokenUserPhoneValue{
				Value: "1234567890",
			},
			Name: &billingpb.TokenUserValue{
				Value: "Unit Test",
			},
			Ip: &billingpb.TokenUserIpValue{
				Value: "127.0.0.1",
			},
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "ru",
			},
			Address: &billingpb.OrderBillingAddress{
				Country:    "RU",
				City:       "St.Petersburg",
				PostalCode: "190000",
				State:      "SPE",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   project.Id,
			Currency:    "RUB",
			Amount:      100,
			Description: "test payment",
		},
	}
	customer, err := suite.service.createCustomer(context.TODO(), customerRequest, project)
	if err != nil {
		suite.FailNow("Create customer failed", "%v", err)
	}

	browserCustomer := &BrowserCookieCustomer{
		CustomerId: customer.Id,
		Ip:         "127.0.0.1",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	cookie, err := suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), cookie)

	order := HelperCreateAndPayOrder(
		suite.Suite,
		suite.service,
		100,
		"RUB",
		"RU",
		project,
		suite.paymentMethod,
		cookie,
	)

	to := now.Monday().Add(-1 * time.Nanosecond)
	date := to.Add(-time.Duration(suite.service.cfg.RoyaltyReportPeriod/2) * time.Second)

	order.PaymentMethodOrderClosedAt, _ = ptypes.TimestampProto(date)
	err = suite.service.updateOrder(context.TODO(), order)
	if !assert.NoError(suite.T(), err) {
		suite.FailNow("update order failed", "%v", err)
	}

	return order
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_CreateRoyaltyReport_Ok_MerchantWithCorrectionAndReserve() {
	projects := []*billingpb.Project{suite.project, suite.project1}

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	entryDate := now.Monday().Add(-1 * time.Nanosecond)

	req := &billingpb.CreateAccountingEntryRequest{
		Type:       pkg.AccountingEntryTypeMerchantRoyaltyCorrection,
		MerchantId: suite.merchant.Id,
		Amount:     10,
		Currency:   suite.merchant.GetPayoutCurrency(),
		Status:     pkg.BalanceTransactionStatusAvailable,
		Date:       entryDate.Unix(),
		Reason:     "unit test",
	}
	rsp := &billingpb.CreateAccountingEntryResponse{}
	err = suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	req = &billingpb.CreateAccountingEntryRequest{
		Type:       pkg.AccountingEntryTypeMerchantRollingReserveCreate,
		MerchantId: suite.merchant.Id,
		Amount:     100,
		Currency:   suite.merchant.GetPayoutCurrency(),
		Status:     pkg.BalanceTransactionStatusAvailable,
		Date:       entryDate.Unix(),
		Reason:     "unit test",
	}
	err = suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	reqReport := &billingpb.CreateRoyaltyReportRequest{
		Merchants: []string{suite.project.GetMerchantId()},
	}
	rspReport := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), reqReport, rspReport)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rspReport.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), reports)
	assert.Len(suite.T(), reports, len(rspReport.Merchants))

	report := reports[0]
	assert.Len(suite.T(), report.Summary.ProductsItems, 1)
	assert.Len(suite.T(), report.Summary.Corrections, 1)
	assert.Len(suite.T(), report.Summary.RollingReserves, 1)
	assert.Equal(suite.T(), report.Totals.RollingReserveAmount, float64(100))
	assert.Equal(suite.T(), report.Totals.CorrectionAmount, float64(10))
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_GetRoyaltyReport_Ok() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)

	req1 := &billingpb.GetRoyaltyReportRequest{
		ReportId:   reports[0].Id,
		MerchantId: reports[0].MerchantId,
	}
	rsp1 := &billingpb.GetRoyaltyReportResponse{}
	err = suite.service.GetRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotEmpty(suite.T(), rsp1.Item)
	assert.EqualValues(suite.T(), rsp1.Item.Id, reports[0].Id)
	assert.EqualValues(suite.T(), rsp1.Item.MerchantId, reports[0].MerchantId)
	assert.EqualValues(suite.T(), rsp1.Item.Status, reports[0].Status)
	assert.EqualValues(suite.T(), rsp1.Item.OperatingCompanyId, reports[0].OperatingCompanyId)
	assert.EqualValues(suite.T(), rsp1.Item.Currency, reports[0].Currency)
	assert.EqualValues(suite.T(), rsp1.Item.DisputeReason, reports[0].DisputeReason)
	assert.EqualValues(suite.T(), rsp1.Item.IsAutoAccepted, reports[0].IsAutoAccepted)
	assert.EqualValues(suite.T(), rsp1.Item.PayoutDocumentId, reports[0].PayoutDocumentId)
	assert.EqualValues(suite.T(), rsp1.Item.Totals, reports[0].Totals)
	assert.EqualValues(suite.T(), rsp1.Item.Summary, reports[0].Summary)
	assert.EqualValues(suite.T(), rsp1.Item.CreatedAt.Seconds, reports[0].CreatedAt.Seconds)
	assert.EqualValues(suite.T(), rsp1.Item.UpdatedAt.Seconds, reports[0].UpdatedAt.Seconds)
	assert.EqualValues(suite.T(), rsp1.Item.AcceptExpireAt.Seconds, reports[0].AcceptExpireAt.Seconds)
	assert.EqualValues(suite.T(), rsp1.Item.PeriodFrom.Seconds, reports[0].PeriodFrom.Seconds)
	assert.EqualValues(suite.T(), rsp1.Item.PeriodTo.Seconds, reports[0].PeriodTo.Seconds)
	assert.Empty(suite.T(), rsp1.Item.AcceptedAt)
	assert.Empty(suite.T(), rsp1.Item.DisputeClosedAt)
	assert.Empty(suite.T(), rsp1.Item.DisputeStartedAt)
	assert.Empty(suite.T(), rsp1.Item.PayoutDate)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_CreateRoyaltyReport_OnlyTestOrders_Ok() {
	projects := []*billingpb.Project{suite.project3}

	for _, v := range projects {
		for i := 0; i < 5; i++ {
			suite.createOrder(v)
		}
	}
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), reports)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ChangeRoyaltyReport_DisputeAndCorrection_SendEmail_MerchantNotFound_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	report, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.NotNil(suite.T(), report[0])
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, report[0].Status)
	assert.EqualValues(suite.T(), -62135596800, report[0].AcceptedAt.Seconds)
	assert.Len(suite.T(), report[0].Summary.Corrections, 0)
	assert.Equal(suite.T(), report[0].Totals.CorrectionAmount, float64(0))

	merchant, err := suite.service.merchantRepository.GetById(context.TODO(), report[0].MerchantId)
	assert.NoError(suite.T(), err)
	counter := 0
	merchantRepositoryMockGetByIdFn := func(ctx context.Context, id string) error {
		if counter <= 0 {
			counter++
			return nil
		}
		counter++
		return errors.New("some error")
	}

	merchantRepositoryMock := &mocks.MerchantRepositoryInterface{}
	merchantRepositoryMock.On("GetById", mock.Anything, mock.Anything).
		Return(merchant, merchantRepositoryMockGetByIdFn)
	suite.service.merchantRepository = merchantRepositoryMock

	req1 := &billingpb.MerchantReviewRoyaltyReportRequest{
		ReportId:      report[0].Id,
		IsAccepted:    false,
		DisputeReason: "unit-test",
		Ip:            "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.MerchantReviewRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp1.Status)
	assert.Equal(suite.T(), rsp1.Message, royaltyReportErrorMerchantNotFound)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_ChangeRoyaltyReport_DisputeAndCorrection_SendEmail_MessagePublish_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	report, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.NotNil(suite.T(), report[0])
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, report[0].Status)
	assert.EqualValues(suite.T(), -62135596800, report[0].AcceptedAt.Seconds)
	assert.Len(suite.T(), report[0].Summary.Corrections, 0)
	assert.Equal(suite.T(), report[0].Totals.CorrectionAmount, float64(0))

	brokerMock := &mocks.BrokerInterface{}
	brokerMock.On("Publish", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("some error"))
	suite.service.postmarkBroker = brokerMock

	req1 := &billingpb.MerchantReviewRoyaltyReportRequest{
		ReportId:      report[0].Id,
		IsAccepted:    false,
		DisputeReason: "unit-test",
		Ip:            "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.MerchantReviewRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp1.Status)
	assert.Equal(suite.T(), rsp1.Message, royaltyReportEntryErrorUnknown)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_RoyaltyReportFinanceDone_Ok() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	report, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.NotNil(suite.T(), report[0])

	emailsCounter := 0
	postmarkBrokerMockFn := func(_ string, _ proto.Message, _ amqp.Table) error {
		emailsCounter++
		return nil
	}
	postmarkBrokerMock := &mocks.BrokerInterface{}
	postmarkBrokerMock.On("Publish", postmarkpb.PostmarkSenderTopicName, mock.Anything, mock.Anything).
		Return(postmarkBrokerMockFn)
	suite.service.postmarkBroker = postmarkBrokerMock

	t := time.Now()
	req1 := &billingpb.ReportFinanceDoneRequest{
		RoyaltyReportId:        report[0].Id,
		MerchantId:             report[0].MerchantId,
		PeriodFrom:             t.Format(billingpb.FilterDateFormat),
		PeriodTo:               t.Format(billingpb.FilterDateFormat),
		LicenseAgreementNumber: "ace2fc5c-b8c2-4424-96e8-5b631a73b88a",
		OperatingCompanyName:   "Company Name",
		FileName:               "file_name.txt",
		FileContent:            []byte(``),
	}
	rsp1 := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.RoyaltyReportFinanceDone(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Zero(suite.T(), emailsCounter)

	attachments := suite.service.royaltyReportRepository.GetAllRoyaltyReportFinanceItems(req1.RoyaltyReportId)
	assert.Len(suite.T(), attachments, 1)
	assert.Equal(suite.T(), req1.FileName, attachments[0].Name)
	assert.Equal(suite.T(), base64.StdEncoding.EncodeToString(req1.FileContent), attachments[0].Content)

	req1.FileName = "file_name1.txt"
	err = suite.service.RoyaltyReportFinanceDone(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.EqualValues(suite.T(), 1, emailsCounter)

	attachments = suite.service.royaltyReportRepository.GetAllRoyaltyReportFinanceItems(req1.RoyaltyReportId)
	assert.Empty(suite.T(), attachments)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_RoyaltyReportFinanceDone_RoyaltyReportNotFound_Error() {
	t := time.Now()
	req := &billingpb.ReportFinanceDoneRequest{
		RoyaltyReportId:        "ffffffffffffffffffffffff",
		MerchantId:             "ffffffffffffffffffffffff",
		PeriodFrom:             t.Format(billingpb.FilterDateFormat),
		PeriodTo:               t.Format(billingpb.FilterDateFormat),
		LicenseAgreementNumber: "ace2fc5c-b8c2-4424-96e8-5b631a73b88a",
		OperatingCompanyName:   "Company Name",
		FileName:               "file_name.txt",
		FileContent:            []byte(``),
	}
	rsp := &billingpb.EmptyResponseWithStatus{}
	err := suite.service.RoyaltyReportFinanceDone(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), royaltyReportErrorReportNotFound, rsp.Message)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_RoyaltyReportFinanceDone_SetRoyaltyReportFinanceItem_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	report, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.NotNil(suite.T(), report[0])

	royaltyReportRepositoryMock := &mocks.RoyaltyReportRepositoryInterface{}
	royaltyReportRepositoryMock.On("GetById", mock2.Anything, mock2.Anything).
		Return(report[0], nil)
	royaltyReportRepositoryMock.On("SetRoyaltyReportFinanceItem", mock2.Anything, mock2.Anything).
		Return(nil, errors.New("SetRoyaltyReportFinanceItem"))
	suite.service.royaltyReportRepository = royaltyReportRepositoryMock

	t := time.Now()
	req1 := &billingpb.ReportFinanceDoneRequest{
		RoyaltyReportId:        report[0].Id,
		MerchantId:             report[0].MerchantId,
		PeriodFrom:             t.Format(billingpb.FilterDateFormat),
		PeriodTo:               t.Format(billingpb.FilterDateFormat),
		LicenseAgreementNumber: "ace2fc5c-b8c2-4424-96e8-5b631a73b88a",
		OperatingCompanyName:   "Company Name",
		FileName:               "file_name.txt",
		FileContent:            []byte(``),
	}
	rsp1 := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.RoyaltyReportFinanceDone(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp1.Status)
	assert.Equal(suite.T(), royaltyReportEntryErrorUnknown, rsp1.Message)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_RoyaltyReportFinanceDone_RemoveRoyaltyReportFinanceItems_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	report, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.NotNil(suite.T(), report[0])

	content := base64.StdEncoding.EncodeToString([]byte(``))

	royaltyReportRepositoryMock := &mocks.RoyaltyReportRepositoryInterface{}
	royaltyReportRepositoryMock.On("GetById", mock2.Anything, mock2.Anything).
		Return(report[0], nil)
	royaltyReportRepositoryMock.On("SetRoyaltyReportFinanceItem", mock2.Anything, mock2.Anything).
		Return(
			[]*postmarkpb.PayloadAttachment{
				{
					Content:     content,
					Name:        "file_name.txt",
					ContentType: "text/plain",
				},
				{
					Content:     content,
					Name:        "file_name.txt",
					ContentType: "text/plain",
				},
			},
			nil,
		)
	royaltyReportRepositoryMock.On("RemoveRoyaltyReportFinanceItems", mock2.Anything).
		Return(errors.New("RemoveRoyaltyReportFinanceItems"))
	suite.service.royaltyReportRepository = royaltyReportRepositoryMock

	t := time.Now()
	req1 := &billingpb.ReportFinanceDoneRequest{
		RoyaltyReportId:        report[0].Id,
		MerchantId:             report[0].MerchantId,
		PeriodFrom:             t.Format(billingpb.FilterDateFormat),
		PeriodTo:               t.Format(billingpb.FilterDateFormat),
		LicenseAgreementNumber: "ace2fc5c-b8c2-4424-96e8-5b631a73b88a",
		OperatingCompanyName:   "Company Name",
		FileName:               "file_name.txt",
		FileContent:            []byte(``),
	}
	rsp1 := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.RoyaltyReportFinanceDone(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp1.Status)
	assert.Equal(suite.T(), royaltyReportEntryErrorUnknown, rsp1.Message)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_RoyaltyReportFinanceDone_PostmarkBroker_Publish_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	report, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), report)
	assert.NotNil(suite.T(), report[0])

	postmarkBrokerMock := &mocks.BrokerInterface{}
	postmarkBrokerMock.On("Publish", postmarkpb.PostmarkSenderTopicName, mock.Anything, mock.Anything).
		Return(errors.New("PostmarkBroker_Publish"))
	suite.service.postmarkBroker = postmarkBrokerMock

	zap.ReplaceGlobals(suite.logObserver)
	_, err = suite.service.royaltyReportRepository.SetRoyaltyReportFinanceItem(
		report[0].Id,
		&postmarkpb.PayloadAttachment{
			Content:     base64.StdEncoding.EncodeToString([]byte(``)),
			Name:        "file_name.txt",
			ContentType: "text/plain",
		},
	)
	assert.NoError(suite.T(), err)

	t := time.Now()
	req1 := &billingpb.ReportFinanceDoneRequest{
		RoyaltyReportId:        report[0].Id,
		MerchantId:             report[0].MerchantId,
		PeriodFrom:             t.Format(billingpb.FilterDateFormat),
		PeriodTo:               t.Format(billingpb.FilterDateFormat),
		LicenseAgreementNumber: "ace2fc5c-b8c2-4424-96e8-5b631a73b88a",
		OperatingCompanyName:   "Company Name",
		FileName:               "file_name1.txt",
		FileContent:            []byte(``),
	}
	rsp1 := &billingpb.EmptyResponseWithStatus{}
	err = suite.service.RoyaltyReportFinanceDone(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp1.Status)
	assert.Equal(suite.T(), royaltyReportEntryErrorUnknown, rsp1.Message)

	logs := suite.zapRecorder.All()
	assert.NotEmpty(suite.T(), logs)
	assert.Equal(suite.T(), zap.ErrorLevel, logs[0].Level)
	assert.Equal(suite.T(), "Publication message to postmark broker failed", logs[0].Message)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_MerchantReviewRoyaltyReport_Accepted_OnRoyaltyReportAccepted_RoyaltyReport_Error() {
	suite.createOrder(suite.project)
	err := suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateRoyaltyReportRequest{}
	rsp := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), reports)
	assert.Equal(suite.T(), billingpb.RoyaltyReportStatusPending, reports[0].Status)
	assert.EqualValues(suite.T(), -62135596800, reports[0].AcceptedAt.Seconds)

	reports[0].Status = billingpb.RoyaltyReportStatusPending
	err = suite.service.royaltyReportRepository.Update(context.TODO(), reports[0], "127.0.0.1", pkg.RoyaltyReportChangeSourceMerchant)
	assert.NoError(suite.T(), err)

	reportingServiceMock := &reportingMocks.ReporterService{}
	reportingServiceMock.On("CreateFile", mock2.Anything, mock2.Anything, mock2.Anything).
		Return(nil, errors.New("ReportingService_CreateFile"))
	suite.service.reporterService = reportingServiceMock

	req1 := &billingpb.MerchantReviewRoyaltyReportRequest{
		ReportId:   reports[0].Id,
		IsAccepted: true,
		Ip:         "127.0.0.1",
	}
	rsp1 := &billingpb.ResponseError{}
	err = suite.service.MerchantReviewRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusSystemError, rsp1.Status)
	assert.Equal(suite.T(), royaltyReportEntryErrorUnknown, rsp1.Message)
}

func (suite *RoyaltyReportTestSuite) TestRoyaltyReport_OnRoyaltyReportAccepted_MerchantNotFound_Error() {
	err := suite.service.onRoyaltyReportStatusChanged(
		context.Background(),
		&billingpb.RoyaltyReport{
			MerchantId: "ffffffffffffffffffffffff",
		},
	)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorNotFound, err)
}
