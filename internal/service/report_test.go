package service

import (
	"context"
	"github.com/globalsign/mgo/bson"
	"github.com/go-redis/redis"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/uuid"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/internal/repository"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	"github.com/paysuper/paysuper-proto/go/postmarkpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/paysuper/paysuper-proto/go/reporterpb"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
	"time"
)

type ReportTestSuite struct {
	suite.Suite
	service *Service
	cache   database.CacheInterface
	log     *zap.Logger

	currencyRub             string
	currencyUsd             string
	project                 *billingpb.Project
	project1                *billingpb.Project
	pmBankCard              *billingpb.PaymentMethod
	pmBitcoin1              *billingpb.PaymentMethod
	customer                *billingpb.Customer
	productIds              []string
	merchantDefaultCurrency string
	cookie                  string

	merchant *billingpb.Merchant
}

func Test_Report(t *testing.T) {
	suite.Run(t, new(ReportTestSuite))
}

func (suite *ReportTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

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

	db, err := mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	broker, err := rabbitmq.NewBroker(cfg.BrokerAddress)
	assert.NoError(suite.T(), err, "Creating RabbitMQ publisher failed")

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
		mocks.NewNotifierOk(),
		mocks.NewBrokerMockOk(),
		mocks.NewBrokerMockOk(),
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.merchant, suite.project, suite.pmBankCard, _, suite.customer = HelperCreateEntitiesForTests(suite.Suite, suite.service)

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
			ProjectId:   suite.project.Id,
			Currency:    "RUB",
			Amount:      100,
			Description: "test payment",
		},
	}
	customer, err := suite.service.createCustomer(context.TODO(), customerRequest, suite.project)
	if err != nil {
		suite.FailNow("Create customer failed", "%v", err)
	}

	browserCustomer := &BrowserCookieCustomer{
		CustomerId: customer.Id,
		Ip:         "127.0.0.1",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	suite.cookie, err = suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), suite.cookie)
}

func (suite *ReportTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *ReportTestSuite) TestReport_ReturnEmptyList() {
	req := &billingpb.ListOrdersRequest{}
	rsp := &billingpb.ListOrdersPublicResponse{}

	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)
	assert.Empty(suite.T(), rsp.Item.Items)

	rsp1 := &billingpb.ListOrdersPrivateResponse{}
	err = suite.service.FindAllOrdersPrivate(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.EqualValues(suite.T(), int32(0), rsp1.Item.Count)
	assert.Empty(suite.T(), rsp1.Item.Items)

	rsp2 := &billingpb.ListOrdersResponse{}
	err = suite.service.FindAllOrders(context.TODO(), req, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.EqualValues(suite.T(), int32(0), rsp1.Item.Count)
	assert.Empty(suite.T(), rsp1.Item.Items)
}

func (suite *ReportTestSuite) TestReport_FindById() {
	req := &billingpb.ListOrdersRequest{Id: uuid.New().String()}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)

	req = &billingpb.ListOrdersRequest{Id: order.Uuid}
	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), int32(1), rsp.Item.Count)
	assert.Equal(suite.T(), order.Id, rsp.Item.Items[0].Id)

	rsp1 := &billingpb.ListOrdersPrivateResponse{}
	err = suite.service.FindAllOrdersPrivate(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.EqualValues(suite.T(), int32(1), rsp1.Item.Count)
	assert.Equal(suite.T(), order.Id, rsp1.Item.Items[0].Id)

	rsp2 := &billingpb.ListOrdersResponse{}
	err = suite.service.FindAllOrders(context.TODO(), req, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp2.Status)
	assert.NotNil(suite.T(), rsp2.Item)
	assert.EqualValues(suite.T(), int32(1), rsp2.Item.Count)
	assert.Equal(suite.T(), order.Id, rsp2.Item.Items[0].Id)
}

func (suite *ReportTestSuite) TestReport_FindByMerchantId() {
	req := &billingpb.ListOrdersRequest{Merchant: []string{suite.project.MerchantId}}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	order1 := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
	order2 := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
	order3 := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)

	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 3, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	var orderIds []string

	for _, v := range rsp.Item.Items {
		orderIds = append(orderIds, v.Id)
	}

	assert.Contains(suite.T(), orderIds, order1.Id)
	assert.Contains(suite.T(), orderIds, order2.Id)
	assert.Contains(suite.T(), orderIds, order3.Id)
}

func (suite *ReportTestSuite) TestReport_FindByProject() {
	req := &billingpb.ListOrdersRequest{Project: []string{suite.project.Id}}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByCountry() {
	req := &billingpb.ListOrdersRequest{Country: []string{"RU"}}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 4; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 4, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByPaymentMethod() {
	req := &billingpb.ListOrdersRequest{PaymentMethod: []string{suite.pmBankCard.Id}}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByStatus() {
	req := &billingpb.ListOrdersRequest{Status: []string{recurringpb.OrderPublicStatusProcessed}}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByAccount() {
	req := &billingpb.ListOrdersRequest{Account: "test@unit.unit"}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), 0, rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	req.Account = "400000"
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}

	req = &billingpb.ListOrdersRequest{QuickSearch: suite.project.Name["en"]}
	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByPmDateFrom() {
	req := &billingpb.ListOrdersRequest{PmDateFrom: time.Now().Add(-10 * time.Second).Format(billingpb.FilterDatetimeFormat)}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByPmDateTo() {
	req := &billingpb.ListOrdersRequest{PmDateTo: time.Now().Add(1000 * time.Second).Format(billingpb.FilterDatetimeFormat)}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string
	date := &timestamp.Timestamp{}

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
		date = order.PaymentMethodOrderClosedAt
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)

	t, err := ptypes.Timestamp(date)
	assert.NoError(suite.T(), err)
	req.PmDateTo = t.Add(100 * time.Second).Format(billingpb.FilterDatetimeFormat)

	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByProjectDateFrom() {
	req := &billingpb.ListOrdersRequest{ProjectDateFrom: time.Now().UTC().Add(-10 * time.Second).Format(billingpb.FilterDatetimeFormat)}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_FindByProjectDateTo() {
	req := &billingpb.ListOrdersRequest{ProjectDateTo: time.Now().Add(100 * time.Second).Format(billingpb.FilterDatetimeFormat)}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	var orderIds []string

	for i := 0; i < 5; i++ {
		order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
		orderIds = append(orderIds, order.Id)
	}

	req.Merchant = append(req.Merchant, suite.project.MerchantId)
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), 5, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, int(rsp.Item.Count))

	for _, v := range rsp.Item.Items {
		assert.Contains(suite.T(), orderIds, v.Id)
	}
}

func (suite *ReportTestSuite) TestReport_GetOrder() {
	req := &billingpb.GetOrderRequest{
		OrderId:    primitive.NewObjectID().Hex(),
		MerchantId: suite.project.MerchantId,
	}
	rsp := &billingpb.GetOrderPublicResponse{}
	err := suite.service.GetOrderPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

	rsp1 := &billingpb.GetOrderPrivateResponse{}
	err = suite.service.GetOrderPrivate(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), orderErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)

	req.OrderId = order.Uuid
	err = suite.service.GetOrderPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)

	err = suite.service.GetOrderPrivate(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
}

func (suite *ReportTestSuite) TestReport_FindByProjectOrderId_QuickSearch_Ok() {
	req := &billingpb.ListOrdersRequest{
		Merchant:    []string{suite.project.MerchantId},
		QuickSearch: "254e3736-000f-5000-8000-178d1d80bf70",
	}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	req1 := &billingpb.OrderCreateRequest{
		Type:        pkg.OrderType_simple,
		ProjectId:   suite.project.Id,
		Amount:      100,
		Currency:    "RUB",
		Account:     "unit test",
		Description: "unit test",
		User: &billingpb.OrderUser{
			Id:    primitive.NewObjectID().Hex(),
			Email: "test@unit.unit",
			Ip:    "127.0.0.1",
			Address: &billingpb.OrderBillingAddress{
				Country:    "RU",
				PostalCode: "19000",
			},
		},
		Metadata: map[string]string{
			"invoiceId": "254e3736-000f-5000-8000-178d1d80bf70",
			"status":    "VIP 8",
			"server":    "3",
		},
	}

	rsp1 := &billingpb.OrderCreateProcessResponse{}
	err = suite.service.OrderCreateProcess(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), rsp.Status, billingpb.ResponseStatusOk)

	_ = HelperPayOrder(suite.Suite, suite.service, rsp1.Item, suite.pmBankCard, "RU", suite.cookie)

	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), 1, rsp.Item.Count)

	// search by quick filter by partial match
	req.QuickSearch = "254e3736-000f"
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), 1, rsp.Item.Count)
}

func (suite *ReportTestSuite) TestReport_FindByMerchantId_QuickSearch_Ok() {
	req := &billingpb.ListOrdersRequest{
		Merchant:    []string{suite.project.MerchantId},
		QuickSearch: suite.merchant.GetCompanyName(),
	}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	expectedCount := 5

	for i := 0; i < expectedCount; i++ {
		_ = HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
	}

	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), expectedCount, rsp.Item.Count)

	// search by quick filter by partial match
	merchantName := suite.merchant.GetCompanyName()
	req.QuickSearch = merchantName[0:4]
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), expectedCount, rsp.Item.Count)
}

func (suite *ReportTestSuite) TestReport_FindByMerchantId_Ok() {
	req := &billingpb.ListOrdersRequest{
		Merchant:     []string{suite.project.MerchantId},
		MerchantName: suite.merchant.GetCompanyName(),
	}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err := suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), int32(0), rsp.Item.Count)

	expectedCount := 3

	for i := 0; i < expectedCount; i++ {
		_ = HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
	}

	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), expectedCount, rsp.Item.Count)

	// search by quick filter by partial match
	merchantName := suite.merchant.GetCompanyName()
	req.MerchantName = merchantName[0:4]
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.EqualValues(suite.T(), expectedCount, rsp.Item.Count)
}

func (suite *ReportTestSuite) TestReport_FindByRoyaltyReportId_Ok() {
	suite.project.Status = billingpb.ProjectStatusInProduction
	err := suite.service.project.Update(context.TODO(), suite.project)
	assert.NoError(suite.T(), err)

	expectedCount := 5
	for i := 0; i < expectedCount; i++ {
		_ = HelperCreateAndPayOrder(suite.Suite, suite.service, 555.55, "RUB", "RU", suite.project, suite.pmBankCard, suite.cookie)
	}

	datePast := now.Monday().Add(-1 * time.Hour)
	diff := datePast.Sub(time.Now())

	res, err := suite.service.db.Collection(repository.CollectionOrder).UpdateMany(
		context.TODO(),
		bson.M{},
		bson.M{"$set": bson.M{"pm_order_close_date": time.Now().Add(diff)}},
	)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), expectedCount, res.MatchedCount)
	assert.EqualValues(suite.T(), expectedCount, res.ModifiedCount)
	err = suite.service.updateOrderView(context.TODO(), []string{})
	assert.NoError(suite.T(), err)

	reporterMock := &reportingMocks.ReporterService{}
	reporterMock.On("CreateFile", mock.Anything, mock.Anything, mock.Anything).
		Return(&reporterpb.CreateFileResponse{Status: billingpb.ResponseStatusOk}, nil)
	suite.service.reporterService = reporterMock

	postmarkBrokerMock := &mocks.BrokerInterface{}
	postmarkBrokerMock.On("Publish", postmarkpb.PostmarkSenderTopicName, mock.Anything, mock.Anything).Return(nil, nil)
	suite.service.postmarkBroker = postmarkBrokerMock

	req1 := &billingpb.CreateRoyaltyReportRequest{}
	rsp1 := &billingpb.CreateRoyaltyReportRequest{}
	err = suite.service.CreateRoyaltyReport(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), rsp1.Merchants)

	reports, err := suite.service.royaltyReportRepository.GetAll(context.TODO())
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), reports)
	assert.Len(suite.T(), reports, 1)

	req := &billingpb.ListOrdersRequest{RoyaltyReportId: reports[0].Id}
	rsp := &billingpb.ListOrdersPublicResponse{}
	err = suite.service.FindAllOrdersPublic(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), expectedCount, rsp.Item.Count)
	assert.Len(suite.T(), rsp.Item.Items, expectedCount)
}

func (suite *ReportTestSuite) TestReport_FindByInvoiceId_Ok() {
	invoiceId := uuid.New().String()
	metadata := map[string]string{"invoiceId": invoiceId}
	_ = HelperCreateAndPayOrder2(
		suite.Suite,
		suite.service,
		555.55,
		"RUB",
		"RU",
		suite.project,
		suite.pmBankCard,
		time.Now(),
		nil,
		nil,
		"",
		metadata,
		suite.cookie,
	)

	req := &billingpb.ListOrdersRequest{
		InvoiceId: invoiceId,
		Merchant:  []string{suite.project.MerchantId},
	}
	rsp := &billingpb.ListOrdersResponse{}
	err := suite.service.FindAllOrders(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), int32(1), rsp.Item.Count)
	assert.Equal(suite.T(), metadata, rsp.Item.Items[0].Metadata)

	invoiceId = uuid.New().String()
	metadata = map[string]string{"order_identifier": invoiceId}
	_ = HelperCreateAndPayOrder2(
		suite.Suite,
		suite.service,
		555.55,
		"RUB",
		"RU",
		suite.project,
		suite.pmBankCard,
		time.Now(),
		nil,
		nil,
		"",
		metadata,
		suite.cookie,
	)

	req.InvoiceId = invoiceId
	err = suite.service.FindAllOrders(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), int32(1), rsp.Item.Count)
	assert.Equal(suite.T(), metadata, rsp.Item.Items[0].Metadata)

	invoiceId = uuid.New().String()
	metadata = map[string]string{"invoiceId": invoiceId}
	order := HelperCreateAndPayOrder2(
		suite.Suite,
		suite.service,
		555.55,
		"RUB",
		"RU",
		suite.project,
		suite.pmBankCard,
		time.Now(),
		nil,
		nil,
		"",
		metadata,
		suite.cookie,
	)
	order.IsProduction = true
	err = suite.service.orderRepository.Update(context.Background(), order)
	assert.NoError(suite.T(), err)

	req.InvoiceId = invoiceId
	req.HideTest = true
	err = suite.service.FindAllOrders(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.EqualValues(suite.T(), int32(1), rsp.Item.Count)
	assert.Equal(suite.T(), metadata, rsp.Item.Items[0].Metadata)
}
