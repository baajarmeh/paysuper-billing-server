package service

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/internal/repository"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	casbinMocks "github.com/paysuper/paysuper-proto/go/casbinpb/mocks"
	reportingMocks "github.com/paysuper/paysuper-proto/go/reporterpb/mocks"
	tools "github.com/paysuper/paysuper-tools/number"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
	"time"
)

type AccountingEntryTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger
	cache   database.CacheInterface

	projectFixedAmount *billingpb.Project
	paymentMethod      *billingpb.PaymentMethod
	paymentSystem      *billingpb.PaymentSystem
	merchant           *billingpb.Merchant
	customer           *billingpb.Customer
	cookie             string
}

var ctx = context.TODO()

func Test_AccountingEntry(t *testing.T) {
	suite.Run(t, new(AccountingEntryTestSuite))
}

func (suite *AccountingEntryTestSuite) SetupTest() {
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
		broker,
		&casbinMocks.CasbinService{},
		nil,
		mocks.NewBrokerMockOk(),
		mocks.NewBrokerMockOk(),
	)

	if err := suite.service.Init(); err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.merchant, suite.projectFixedAmount, suite.paymentMethod, suite.paymentSystem, suite.customer = HelperCreateEntitiesForTests(suite.Suite, suite.service)

	browserCustomer := &BrowserCookieCustomer{
		CustomerId: suite.customer.Id,
		Ip:         "127.0.0.1",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	suite.cookie, err = suite.service.generateBrowserCookie(browserCustomer)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), suite.cookie)
}

func (suite *AccountingEntryTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Ok_RUB_RUB_RUB() {
	// Order currency RUB
	// Royalty currency RUB
	// VAT currency RUB

	req := &billingpb.GetMerchantByRequest{
		MerchantId: suite.projectFixedAmount.MerchantId,
	}
	rsp := &billingpb.GetMerchantResponse{}
	err := suite.service.GetMerchantBy(ctx, req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)

	merchant := rsp.Item
	merchant.Banking.Currency = "RUB"
	err = suite.service.merchantRepository.Update(ctx, merchant)
	assert.Nil(suite.T(), err)

	orderAmount := float64(100)
	orderCountry := "RU"
	orderCurrency := "RUB"
	orderControlResults := map[string]float64{
		"real_gross_revenue":                        120,
		"real_tax_fee":                              20,
		"central_bank_tax_fee":                      0,
		"real_tax_fee_total":                        20,
		"ps_gross_revenue_fx":                       0,
		"ps_gross_revenue_fx_tax_fee":               0,
		"ps_gross_revenue_fx_profit":                0,
		"merchant_gross_revenue":                    120,
		"merchant_tax_fee_cost_value":               20,
		"merchant_tax_fee_central_bank_fx":          0,
		"merchant_tax_fee":                          20,
		"ps_method_fee":                             6,
		"merchant_method_fee":                       3,
		"merchant_method_fee_cost_value":            1.8,
		"ps_markup_merchant_method_fee":             1.2,
		"merchant_method_fixed_fee":                 1.469388,
		"real_merchant_method_fixed_fee":            1.44,
		"markup_merchant_method_fixed_fee_fx":       0.029388,
		"real_merchant_method_fixed_fee_cost_value": 0.65,
		"ps_method_fixed_fee_profit":                0.79,
		"merchant_ps_fixed_fee":                     3.673469,
		"real_merchant_ps_fixed_fee":                3.6,
		"markup_merchant_ps_fixed_fee":              0.073469,
		"ps_method_profit":                          7.223469,
		"merchant_net_revenue":                      90.326531,
		"ps_profit_total":                           7.223469,
	}

	refundControlResults := map[string]float64{
		"real_refund":                          120,
		"real_refund_tax_fee":                  20,
		"real_refund_fee":                      12,
		"real_refund_fixed_fee":                10.8,
		"merchant_refund":                      120,
		"ps_merchant_refund_fx":                0,
		"merchant_refund_fee":                  0,
		"ps_markup_merchant_refund_fee":        -12,
		"merchant_refund_fixed_fee_cost_value": 0,
		"merchant_refund_fixed_fee":            0,
		"ps_merchant_refund_fixed_fee_fx":      0,
		"ps_merchant_refund_fixed_fee_profit":  -10.8,
		"reverse_tax_fee":                      20,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             20,
		"merchant_reverse_revenue":             100,
		"ps_refund_profit":                     -22.8,
	}

	assert.GreaterOrEqual(suite.T(), orderControlResults["real_gross_revenue"], orderControlResults["merchant_gross_revenue"])

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err = suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, false)
	assert.NotNil(suite.T(), refund)

	accountingEntries := suite.helperGetAccountingEntries(order.Id, repository.CollectionOrder)
	assert.Equal(suite.T(), len(accountingEntries), len(orderControlResults)-11)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "RUB")
	for _, entry := range accountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, orderControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, orderControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"] + orderControlResults["ps_gross_revenue_fx"]
	assert.Equal(suite.T(), orderControlResults["real_gross_revenue"], tools.ToPrecise(controlRealGrossRevenue))

	controlMerchantGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"]
	assert.Equal(suite.T(), orderControlResults["merchant_gross_revenue"], tools.ToPrecise(controlMerchantGrossRevenue))

	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "RUB")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)
	suite.helperCheckOrderView(order.Id, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, orderControlResults)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Ok_RUB_USD_RUB() {
	// Order currency RUB
	// Royalty currency USD
	// VAT currency RUB

	orderAmount := float64(650)
	orderCountry := "RU"
	orderCurrency := "RUB"
	orderControlResults := map[string]float64{
		"real_gross_revenue":                        12.0003,
		"real_tax_fee":                              2.00005,
		"central_bank_tax_fee":                      0,
		"real_tax_fee_total":                        2.00005,
		"ps_gross_revenue_fx":                       0.24492,
		"ps_gross_revenue_fx_tax_fee":               0.04082,
		"ps_gross_revenue_fx_profit":                0.2041,
		"merchant_gross_revenue":                    11.75538,
		"merchant_tax_fee_cost_value":               1.95923,
		"merchant_tax_fee_central_bank_fx":          0.045919,
		"merchant_tax_fee":                          2.005149,
		"ps_method_fee":                             0.587769,
		"merchant_method_fee":                       0.293885,
		"merchant_method_fee_cost_value":            0.180004,
		"ps_markup_merchant_method_fee":             0.113881,
		"merchant_method_fixed_fee":                 0.022606,
		"real_merchant_method_fixed_fee":            0.022154,
		"markup_merchant_method_fixed_fee_fx":       0.000452,
		"real_merchant_method_fixed_fee_cost_value": 0.01,
		"ps_method_fixed_fee_profit":                0.012154,
		"merchant_ps_fixed_fee":                     0.056515,
		"real_merchant_ps_fixed_fee":                0.055385,
		"markup_merchant_ps_fixed_fee":              0.00113,
		"ps_method_profit":                          0.45428,
		"merchant_net_revenue":                      9.105947,
		"ps_profit_total":                           0.65838,
	}

	refundControlResults := map[string]float64{
		"real_refund":                          12.0003,
		"real_refund_tax_fee":                  2.00005,
		"real_refund_fee":                      1.20003,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      12.24522,
		"ps_merchant_refund_fx":                0.24492,
		"merchant_refund_fee":                  0,
		"ps_markup_merchant_refund_fee":        -1.20003,
		"merchant_refund_fixed_fee_cost_value": 0,
		"merchant_refund_fixed_fee":            0,
		"ps_merchant_refund_fixed_fee_fx":      0,
		"ps_merchant_refund_fixed_fee_profit":  -0.166154,
		"reverse_tax_fee":                      2.005149,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0.001914,
		"merchant_reverse_tax_fee":             2.005149,
		"merchant_reverse_revenue":             10.240071,
		"ps_refund_profit":                     -1.36427,
	}

	assert.GreaterOrEqual(suite.T(), orderControlResults["real_gross_revenue"], orderControlResults["merchant_gross_revenue"])

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, false)
	assert.NotNil(suite.T(), refund)

	orderAccountingEntries := suite.helperGetAccountingEntries(order.Id, repository.CollectionOrder)
	assert.Equal(suite.T(), len(orderAccountingEntries), len(orderControlResults)-11)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range orderAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, orderControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, orderControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"] + orderControlResults["ps_gross_revenue_fx"]
	assert.Equal(suite.T(), orderControlResults["real_gross_revenue"], tools.ToPrecise(controlRealGrossRevenue))

	controlMerchantGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"]
	assert.Equal(suite.T(), orderControlResults["merchant_gross_revenue"], tools.ToPrecise(controlMerchantGrossRevenue))

	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)
	suite.helperCheckOrderView(order.Id, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, orderControlResults)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Ok_RUB_USD_USD() {
	// Order currency RUB
	// Royalty currency USD
	// VAT currency USD

	orderAmount := float64(650)
	orderCountry := "US"
	orderCurrency := "RUB"
	orderControlResults := map[string]float64{
		"real_gross_revenue":                        11.900297,
		"real_tax_fee":                              1.900048,
		"central_bank_tax_fee":                      0,
		"real_tax_fee_total":                        1.900048,
		"ps_gross_revenue_fx":                       0.24288,
		"ps_gross_revenue_fx_tax_fee":               0.038779,
		"ps_gross_revenue_fx_profit":                0.204101,
		"merchant_gross_revenue":                    11.657417,
		"merchant_tax_fee_cost_value":               1.861268,
		"merchant_tax_fee_central_bank_fx":          0,
		"merchant_tax_fee":                          1.861268,
		"ps_method_fee":                             0.582871,
		"merchant_method_fee":                       0.291435,
		"merchant_method_fee_cost_value":            0.178504,
		"ps_markup_merchant_method_fee":             0.112931,
		"merchant_method_fixed_fee":                 0.022606,
		"real_merchant_method_fixed_fee":            0.022154,
		"markup_merchant_method_fixed_fee_fx":       0.000452,
		"real_merchant_method_fixed_fee_cost_value": 0.01,
		"ps_method_fixed_fee_profit":                0.012154,
		"merchant_ps_fixed_fee":                     0.056515,
		"real_merchant_ps_fixed_fee":                0.055385,
		"markup_merchant_ps_fixed_fee":              0.00113,
		"ps_method_profit":                          0.450882,
		"merchant_net_revenue":                      9.156763,
		"ps_profit_total":                           0.654983,
	}

	refundControlResults := map[string]float64{
		"real_refund":                          11.900297,
		"real_refund_tax_fee":                  1.900048,
		"real_refund_fee":                      1.19003,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      12.143177,
		"ps_merchant_refund_fx":                0.24288,
		"merchant_refund_fee":                  0,
		"ps_markup_merchant_refund_fee":        -1.19003,
		"merchant_refund_fixed_fee_cost_value": 0,
		"merchant_refund_fixed_fee":            0,
		"ps_merchant_refund_fixed_fee_fx":      0,
		"ps_merchant_refund_fixed_fee_profit":  -0.166154,
		"reverse_tax_fee":                      1.861268,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             1.861268,
		"merchant_reverse_revenue":             10.281909,
		"ps_refund_profit":                     -1.356184,
	}

	assert.GreaterOrEqual(suite.T(), orderControlResults["real_gross_revenue"], orderControlResults["merchant_gross_revenue"])

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, false)
	assert.NotNil(suite.T(), refund)

	orderAccountingEntries := suite.helperGetAccountingEntries(order.Id, repository.CollectionOrder)
	assert.Equal(suite.T(), len(orderAccountingEntries), len(orderControlResults)-11)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range orderAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, orderControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, orderControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"] + orderControlResults["ps_gross_revenue_fx"]
	assert.Equal(suite.T(), orderControlResults["real_gross_revenue"], tools.ToPrecise(controlRealGrossRevenue))

	controlMerchantGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"]
	assert.Equal(suite.T(), orderControlResults["merchant_gross_revenue"], tools.ToPrecise(controlMerchantGrossRevenue))

	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)
	suite.helperCheckOrderView(order.Id, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, orderControlResults)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Ok_RUB_USD_EUR_VatPayer_Buyer() {
	// Order currency RUB
	// Royalty currency USD
	// VAT currency EUR

	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"
	orderControlResults := map[string]float64{
		"real_gross_revenue":                        12.0003,
		"real_tax_fee":                              2.00005,
		"central_bank_tax_fee":                      0,
		"real_tax_fee_total":                        2.00005,
		"ps_gross_revenue_fx":                       0.24492,
		"ps_gross_revenue_fx_tax_fee":               0.04082,
		"ps_gross_revenue_fx_profit":                0.2041,
		"merchant_gross_revenue":                    11.75538,
		"merchant_tax_fee_cost_value":               1.95923,
		"merchant_tax_fee_central_bank_fx":          0,
		"merchant_tax_fee":                          1.95923,
		"ps_method_fee":                             0.587769,
		"merchant_method_fee":                       0.293885,
		"merchant_method_fee_cost_value":            0.180004,
		"ps_markup_merchant_method_fee":             0.113881,
		"merchant_method_fixed_fee":                 0.022606,
		"real_merchant_method_fixed_fee":            0.022154,
		"markup_merchant_method_fixed_fee_fx":       0.000452,
		"real_merchant_method_fixed_fee_cost_value": 0.01,
		"ps_method_fixed_fee_profit":                0.012154,
		"merchant_ps_fixed_fee":                     0.056515,
		"real_merchant_ps_fixed_fee":                0.055385,
		"markup_merchant_ps_fixed_fee":              0.00113,
		"ps_method_profit":                          0.45428,
		"merchant_net_revenue":                      9.151866,
		"ps_profit_total":                           0.65838,
	}

	refundControlResults := map[string]float64{
		"real_refund":                          12.0003,
		"real_refund_tax_fee":                  2.00005,
		"real_refund_fee":                      1.20003,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      12.24522,
		"ps_merchant_refund_fx":                0.24492,
		"merchant_refund_fee":                  0,
		"ps_markup_merchant_refund_fee":        -1.20003,
		"merchant_refund_fixed_fee_cost_value": 0,
		"merchant_refund_fixed_fee":            0,
		"ps_merchant_refund_fixed_fee_fx":      0,
		"ps_merchant_refund_fixed_fee_profit":  -0.166154,
		"reverse_tax_fee":                      1.95923,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             1.95923,
		"merchant_reverse_revenue":             10.28599,
		"ps_refund_profit":                     -1.366184,
	}

	assert.GreaterOrEqual(suite.T(), orderControlResults["real_gross_revenue"], orderControlResults["merchant_gross_revenue"])

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)
	assert.Equal(suite.T(), order.VatPayer, billingpb.VatPayerBuyer)
	assert.NotNil(suite.T(), order.Tax)
	assert.Equal(suite.T(), order.Tax.Currency, "RUB")
	assert.EqualValues(suite.T(), order.Tax.Rate, 0.2)
	assert.EqualValues(suite.T(), order.Tax.Amount, 130)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, false)
	assert.NotNil(suite.T(), refund)

	orderAccountingEntries := suite.helperGetAccountingEntries(order.Id, repository.CollectionOrder)
	assert.Equal(suite.T(), len(orderAccountingEntries), len(orderControlResults)-11)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range orderAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, orderControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, orderControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"] + orderControlResults["ps_gross_revenue_fx"]
	assert.Equal(suite.T(), orderControlResults["real_gross_revenue"], tools.ToPrecise(controlRealGrossRevenue))

	controlMerchantGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"]
	assert.Equal(suite.T(), orderControlResults["merchant_gross_revenue"], tools.ToPrecise(controlMerchantGrossRevenue))

	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)
	suite.helperCheckOrderView(order.Id, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, orderControlResults)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Ok_RUB_USD_EUR_VatPayer_Seller() {
	project := HelperCreateProject(suite.Suite, suite.service, suite.merchant.Id, billingpb.VatPayerSeller)

	// Order currency RUB
	// Royalty currency USD
	// VAT currency EUR

	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"
	orderControlResults := map[string]float64{
		"real_gross_revenue":                        10.00025,
		"real_tax_fee":                              1.666657,
		"central_bank_tax_fee":                      0,
		"real_tax_fee_total":                        1.666657,
		"ps_gross_revenue_fx":                       0.2041,
		"ps_gross_revenue_fx_tax_fee":               0.034017,
		"ps_gross_revenue_fx_profit":                0.170083,
		"merchant_gross_revenue":                    9.79615,
		"merchant_tax_fee_cost_value":               1.632692,
		"merchant_tax_fee_central_bank_fx":          0,
		"merchant_tax_fee":                          1.632692,
		"ps_method_fee":                             0.489807,
		"merchant_method_fee":                       0.244904,
		"merchant_method_fee_cost_value":            0.150004,
		"ps_markup_merchant_method_fee":             0.0949,
		"merchant_method_fixed_fee":                 0.022606,
		"real_merchant_method_fixed_fee":            0.022154,
		"markup_merchant_method_fixed_fee_fx":       0.000452,
		"real_merchant_method_fixed_fee_cost_value": 0.01,
		"ps_method_fixed_fee_profit":                0.012154,
		"merchant_ps_fixed_fee":                     0.056515,
		"real_merchant_ps_fixed_fee":                0.055385,
		"markup_merchant_ps_fixed_fee":              0.00113,
		"ps_method_profit":                          0.386318,
		"merchant_net_revenue":                      7.617136,
		"ps_profit_total":                           0.556401,
	}

	refundControlResults := map[string]float64{
		"real_refund":                          10.00025,
		"real_refund_tax_fee":                  1.666657,
		"real_refund_fee":                      1.000025,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      10.20435,
		"ps_merchant_refund_fx":                0.2041,
		"merchant_refund_fee":                  0,
		"ps_markup_merchant_refund_fee":        -1.000025,
		"merchant_refund_fixed_fee_cost_value": 0,
		"merchant_refund_fixed_fee":            0,
		"ps_merchant_refund_fixed_fee_fx":      0,
		"ps_merchant_refund_fixed_fee_profit":  -0.166154,
		"reverse_tax_fee":                      1.632692,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             1.632692,
		"merchant_reverse_revenue":             8.571658,
		"ps_refund_profit":                     -1.166179,
	}

	assert.GreaterOrEqual(suite.T(), orderControlResults["real_gross_revenue"], orderControlResults["merchant_gross_revenue"])

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, project, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)
	assert.Equal(suite.T(), order.VatPayer, billingpb.VatPayerSeller)
	assert.NotNil(suite.T(), order.Tax)
	assert.Equal(suite.T(), order.Tax.Currency, "RUB")
	assert.EqualValues(suite.T(), order.Tax.Rate, 0.2)
	assert.EqualValues(suite.T(), order.Tax.Amount, 108.33)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, false)
	assert.NotNil(suite.T(), refund)

	orderAccountingEntries := suite.helperGetAccountingEntries(order.Id, repository.CollectionOrder)
	assert.Equal(suite.T(), len(orderAccountingEntries), len(orderControlResults)-11)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range orderAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, orderControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, orderControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"] + orderControlResults["ps_gross_revenue_fx"]
	assert.Equal(suite.T(), orderControlResults["real_gross_revenue"], tools.ToPrecise(controlRealGrossRevenue))

	controlMerchantGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"]
	assert.Equal(suite.T(), orderControlResults["merchant_gross_revenue"], tools.ToPrecise(controlMerchantGrossRevenue))

	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), tools.ToPrecise(refundControlResults["real_refund"]), tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)
	suite.helperCheckOrderView(order.Id, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, orderControlResults)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Ok_RUB_USD_EUR_VatPayer_Nobody() {
	project := HelperCreateProject(suite.Suite, suite.service, suite.merchant.Id, billingpb.VatPayerNobody)

	// Order currency RUB
	// Royalty currency USD
	// VAT currency EUR

	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"
	orderControlResults := map[string]float64{
		"real_gross_revenue":                        10.00025,
		"real_tax_fee":                              0,
		"central_bank_tax_fee":                      0,
		"real_tax_fee_total":                        0,
		"ps_gross_revenue_fx":                       0.2041,
		"ps_gross_revenue_fx_tax_fee":               0,
		"ps_gross_revenue_fx_profit":                0.2041,
		"merchant_gross_revenue":                    9.79615,
		"merchant_tax_fee_cost_value":               0,
		"merchant_tax_fee_central_bank_fx":          0,
		"merchant_tax_fee":                          0,
		"ps_method_fee":                             0.489807,
		"merchant_method_fee":                       0.244904,
		"merchant_method_fee_cost_value":            0.150004,
		"ps_markup_merchant_method_fee":             0.0949,
		"merchant_method_fixed_fee":                 0.022606,
		"real_merchant_method_fixed_fee":            0.022154,
		"markup_merchant_method_fixed_fee_fx":       0.000452,
		"real_merchant_method_fixed_fee_cost_value": 0.01,
		"ps_method_fixed_fee_profit":                0.012154,
		"merchant_ps_fixed_fee":                     0.056515,
		"real_merchant_ps_fixed_fee":                0.055385,
		"markup_merchant_ps_fixed_fee":              0.00113,
		"ps_method_profit":                          0.386318,
		"merchant_net_revenue":                      9.249828,
		"ps_profit_total":                           0.590418,
	}

	refundControlResults := map[string]float64{
		"real_refund":                          10.00025,
		"real_refund_tax_fee":                  0,
		"real_refund_fee":                      1.000025,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      10.20435,
		"ps_merchant_refund_fx":                0.2041,
		"merchant_refund_fee":                  0,
		"ps_markup_merchant_refund_fee":        -1.000025,
		"merchant_refund_fixed_fee_cost_value": 0,
		"merchant_refund_fixed_fee":            0,
		"ps_merchant_refund_fixed_fee_fx":      0,
		"ps_merchant_refund_fixed_fee_profit":  -0.166154,
		"reverse_tax_fee":                      0,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             0,
		"merchant_reverse_revenue":             10.20435,
		"ps_refund_profit":                     -1.166179,
	}

	assert.GreaterOrEqual(suite.T(), orderControlResults["real_gross_revenue"], orderControlResults["merchant_gross_revenue"])

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, project, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)
	assert.Equal(suite.T(), order.VatPayer, billingpb.VatPayerNobody)
	assert.NotNil(suite.T(), order.Tax)
	assert.Equal(suite.T(), order.Tax.Currency, "RUB")
	assert.EqualValues(suite.T(), order.Tax.Rate, 0)
	assert.EqualValues(suite.T(), order.Tax.Amount, 0)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, false)
	assert.NotNil(suite.T(), refund)

	orderAccountingEntries := suite.helperGetAccountingEntries(order.Id, repository.CollectionOrder)
	assert.Equal(suite.T(), len(orderAccountingEntries), len(orderControlResults)-11)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range orderAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, orderControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, orderControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"] + orderControlResults["ps_gross_revenue_fx"]
	assert.Equal(suite.T(), orderControlResults["real_gross_revenue"], tools.ToPrecise(controlRealGrossRevenue))

	controlMerchantGrossRevenue := orderControlResults["merchant_net_revenue"] + orderControlResults["merchant_ps_fixed_fee"] +
		orderControlResults["ps_method_fee"] + orderControlResults["merchant_tax_fee"]
	assert.Equal(suite.T(), orderControlResults["merchant_gross_revenue"], tools.ToPrecise(controlMerchantGrossRevenue))

	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)
	suite.helperCheckOrderView(order.Id, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, orderControlResults)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Chargeback_Ok_RUB_RUB_RUB() {
	// Order currency RUB
	// Royalty currency RUB
	// VAT currency RUB

	req := &billingpb.GetMerchantByRequest{
		MerchantId: suite.projectFixedAmount.MerchantId,
	}
	rsp := &billingpb.GetMerchantResponse{}
	err := suite.service.GetMerchantBy(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)

	merchant := rsp.Item
	merchant.Banking.Currency = "RUB"
	err = suite.service.merchantRepository.Update(ctx, merchant)
	assert.Nil(suite.T(), err)

	orderAmount := float64(100)
	orderCountry := "RU"
	orderCurrency := "RUB"

	refundControlResults := map[string]float64{
		"real_refund":                          120,
		"real_refund_tax_fee":                  20,
		"real_refund_fee":                      12,
		"real_refund_fixed_fee":                10.8,
		"merchant_refund":                      120,
		"ps_merchant_refund_fx":                0,
		"merchant_refund_fee":                  24,
		"ps_markup_merchant_refund_fee":        12,
		"merchant_refund_fixed_fee_cost_value": 10.8,
		"merchant_refund_fixed_fee":            11.020408,
		"ps_merchant_refund_fixed_fee_fx":      0.220407,
		"ps_merchant_refund_fixed_fee_profit":  0.220407,
		"reverse_tax_fee":                      20,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             20,
		"merchant_reverse_revenue":             135.020408,
		"ps_refund_profit":                     12.220407,
	}

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err = suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, true)
	assert.NotNil(suite.T(), refund)
	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "RUB")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)
	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Chargeback_Ok_RUB_USD_RUB() {
	// Order currency RUB
	// Royalty currency USD
	// VAT currency RUB

	orderAmount := float64(650)
	orderCountry := "RU"
	orderCurrency := "RUB"

	refundControlResults := map[string]float64{
		"real_refund":                          12.0003,
		"real_refund_tax_fee":                  2.00005,
		"real_refund_fee":                      1.20003,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      12.24522,
		"ps_merchant_refund_fx":                0.24492,
		"merchant_refund_fee":                  2.449044,
		"ps_markup_merchant_refund_fee":        1.249014,
		"merchant_refund_fixed_fee_cost_value": 0.166154,
		"merchant_refund_fixed_fee":            0.169545,
		"ps_merchant_refund_fixed_fee_fx":      0.003391,
		"ps_merchant_refund_fixed_fee_profit":  0.003391,
		"reverse_tax_fee":                      2.005149,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0.001914,
		"merchant_reverse_tax_fee":             2.005149,
		"merchant_reverse_revenue":             12.85866,
		"ps_refund_profit":                     1.254319,
	}

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, true)
	assert.NotNil(suite.T(), refund)
	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Chargeback_Ok_RUB_USD_USD() {
	// Order currency RUB
	// Royalty currency USD
	// VAT currency USD

	orderAmount := float64(650)
	orderCountry := "US"
	orderCurrency := "RUB"

	refundControlResults := map[string]float64{
		"real_refund":                          11.900297,
		"real_refund_tax_fee":                  1.900048,
		"real_refund_fee":                      1.19003,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      12.143177,
		"ps_merchant_refund_fx":                0.24288,
		"merchant_refund_fee":                  2.428635,
		"ps_markup_merchant_refund_fee":        1.238605,
		"merchant_refund_fixed_fee_cost_value": 0.166154,
		"merchant_refund_fixed_fee":            0.169545,
		"ps_merchant_refund_fixed_fee_fx":      0.003391,
		"ps_merchant_refund_fixed_fee_profit":  0.003391,
		"reverse_tax_fee":                      1.861268,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             1.861268,
		"merchant_reverse_revenue":             12.880089,
		"ps_refund_profit":                     1.241996,
	}

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, true)
	assert.NotNil(suite.T(), refund)
	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Chargeback_Ok_RUB_USD_EUR() {
	// Order currency RUB
	// Royalty currency USD
	// VAT currency EUR

	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"

	refundControlResults := map[string]float64{
		"real_refund":                          12.0003,
		"real_refund_tax_fee":                  2.00005,
		"real_refund_fee":                      1.20003,
		"real_refund_fixed_fee":                0.166154,
		"merchant_refund":                      12.24522,
		"ps_merchant_refund_fx":                0.24492,
		"merchant_refund_fee":                  2.449044,
		"ps_markup_merchant_refund_fee":        1.249014,
		"merchant_refund_fixed_fee_cost_value": 0.166154,
		"merchant_refund_fixed_fee":            0.169545,
		"ps_merchant_refund_fixed_fee_fx":      0.003391,
		"ps_merchant_refund_fixed_fee_profit":  0.003391,
		"reverse_tax_fee":                      1.95923,
		"reverse_tax_fee_delta":                0,
		"ps_reverse_tax_fee_delta":             0,
		"merchant_reverse_tax_fee":             1.95923,
		"merchant_reverse_revenue":             12.904579,
		"ps_refund_profit":                     1.252405,
	}

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, true)
	assert.NotNil(suite.T(), refund)
	refundAccountingEntries := suite.helperGetAccountingEntries(refund.CreatedOrderId, repository.CollectionRefund)
	assert.Equal(suite.T(), len(refundAccountingEntries), len(refundControlResults)-7)
	merchantRoyaltyCurrency := order.GetMerchantRoyaltyCurrency()
	assert.Equal(suite.T(), merchantRoyaltyCurrency, "USD")
	for _, entry := range refundAccountingEntries {
		if !assert.Equal(suite.T(), entry.Amount, refundControlResults[entry.Type]) {
			fmt.Println(entry.Type, entry.Amount, refundControlResults[entry.Type])
		}
		assert.Equal(suite.T(), entry.Currency, merchantRoyaltyCurrency)
	}

	controlRealRefund := refundControlResults["merchant_reverse_revenue"] + refundControlResults["merchant_reverse_tax_fee"] -
		refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["merchant_refund_fee"] - refundControlResults["ps_merchant_refund_fx"]
	assert.Equal(suite.T(), refundControlResults["real_refund"], tools.ToPrecise(controlRealRefund))

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)

	refund, err = suite.service.refundRepository.GetById(ctx, refund.Id)
	assert.NoError(suite.T(), err)
	suite.helperCheckRefundView(refund.CreatedOrderId, orderCurrency, merchantRoyaltyCurrency, country.VatCurrency, refundControlResults)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_CreateAccountingEntry_Ok() {
	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, true)
	assert.NotNil(suite.T(), refund)

	req := &billingpb.CreateAccountingEntryRequest{
		Type:       pkg.AccountingEntryTypeRealGrossRevenue,
		OrderId:    order.Id,
		RefundId:   refund.Id,
		MerchantId: order.GetMerchantId(),
		Amount:     10,
		Currency:   "RUB",
		Status:     pkg.BalanceTransactionStatusAvailable,
		Date:       time.Now().Unix(),
		Reason:     "unit test",
	}
	rsp := &billingpb.CreateAccountingEntryResponse{}
	err = suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)

	accountingEntry, err := suite.service.accountingRepository.GetById(ctx, rsp.Item.Id)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), accountingEntry)

	assert.Equal(suite.T(), req.Type, accountingEntry.Type)
	assert.Equal(suite.T(), req.MerchantId, accountingEntry.Source.Id)
	assert.Equal(suite.T(), repository.CollectionMerchant, accountingEntry.Source.Type)
	assert.Equal(suite.T(), req.Amount, accountingEntry.Amount)
	assert.Equal(suite.T(), req.Currency, accountingEntry.Currency)
	assert.Equal(suite.T(), req.Status, accountingEntry.Status)
	assert.Equal(suite.T(), req.Reason, accountingEntry.Reason)

	t, err := ptypes.Timestamp(accountingEntry.CreatedAt)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), req.Date, t.Unix())
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_CreateAccountingEntry_MerchantNotFound_Error() {
	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, true)
	assert.NotNil(suite.T(), refund)

	req := &billingpb.CreateAccountingEntryRequest{
		Type:       pkg.AccountingEntryTypeRealGrossRevenue,
		OrderId:    order.Id,
		RefundId:   refund.Id,
		MerchantId: primitive.NewObjectID().Hex(),
		Amount:     10,
		Currency:   "RUB",
		Status:     pkg.BalanceTransactionStatusAvailable,
		Date:       time.Now().Unix(),
		Reason:     "unit test",
	}

	rsp := &billingpb.CreateAccountingEntryResponse{}
	err = suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), accountingEntryErrorMerchantNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

	aes, err := suite.service.accountingRepository.FindBySource(ctx, req.MerchantId, repository.CollectionMerchant)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), aes)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_CreateAccountingEntry_OrderNotFound_Error() {
	req := &billingpb.CreateAccountingEntryRequest{
		Type:     pkg.AccountingEntryTypeRealGrossRevenue,
		OrderId:  primitive.NewObjectID().Hex(),
		Amount:   10,
		Currency: "RUB",
		Status:   pkg.BalanceTransactionStatusAvailable,
		Date:     time.Now().Unix(),
		Reason:   "unit test",
	}
	rsp := &billingpb.CreateAccountingEntryResponse{}
	err := suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), accountingEntryErrorOrderNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

	aes, err := suite.service.accountingRepository.FindBySource(ctx, req.OrderId, repository.CollectionOrder)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), aes)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_CreateAccountingEntry_RefundNotFound_Error() {
	req := &billingpb.CreateAccountingEntryRequest{
		Type:     pkg.AccountingEntryTypeRealGrossRevenue,
		RefundId: primitive.NewObjectID().Hex(),
		Amount:   10,
		Currency: "RUB",
		Status:   pkg.BalanceTransactionStatusAvailable,
		Date:     time.Now().Unix(),
		Reason:   "unit test",
	}
	rsp := &billingpb.CreateAccountingEntryResponse{}
	err := suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), accountingEntryErrorRefundNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

	aes, err := suite.service.accountingRepository.FindBySource(ctx, req.RefundId, repository.CollectionRefund)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), aes)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_CreateAccountingEntry_Refund_OrderNotFound_Error() {
	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	suite.paymentSystem.Handler = "mock_ok"
	err := suite.service.paymentSystemRepository.Update(ctx, suite.paymentSystem)
	assert.NoError(suite.T(), err)

	refund := HelperMakeRefund(suite.Suite, suite.service, order, order.ChargeAmount, true)
	assert.NotNil(suite.T(), refund)

	refund.OriginalOrder.Id = primitive.NewObjectID().Hex()
	err = suite.service.refundRepository.Update(ctx, refund)
	assert.NoError(suite.T(), err)

	req := &billingpb.CreateAccountingEntryRequest{
		Type:     pkg.AccountingEntryTypeRealGrossRevenue,
		RefundId: refund.Id,
		Amount:   10,
		Currency: "RUB",
		Status:   pkg.BalanceTransactionStatusAvailable,
		Date:     time.Now().Unix(),
		Reason:   "unit test",
	}
	rsp := &billingpb.CreateAccountingEntryResponse{}
	err = suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), accountingEntryErrorOrderNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

	aes, err := suite.service.accountingRepository.FindBySource(ctx, req.RefundId, repository.CollectionRefund)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), aes)
}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_CreateAccountingEntry_EntryNotExist_Error() {
	orderAmount := float64(650)
	orderCountry := "FI"
	orderCurrency := "RUB"

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, suite.projectFixedAmount, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	aes, err := suite.service.accountingRepository.FindBySource(ctx, order.Id, repository.CollectionOrder)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), aes)

	req := &billingpb.CreateAccountingEntryRequest{
		Type:     "not_exist_accounting_entry_name",
		OrderId:  order.Id,
		Amount:   10,
		Currency: "RUB",
		Status:   pkg.BalanceTransactionStatusAvailable,
		Date:     time.Now().Unix(),
		Reason:   "unit test",
	}
	rsp := &billingpb.CreateAccountingEntryResponse{}
	err = suite.service.CreateAccountingEntry(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), accountingEntryErrorUnknownEntry, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

	aes2, err := suite.service.accountingRepository.FindBySource(ctx, order.Id, repository.CollectionOrder)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), aes2)
	assert.EqualValues(suite.T(), aes[0].Id, aes2[0].Id)
}

func (suite *AccountingEntryTestSuite) helperGetAccountingEntries(orderId, collection string) []*billingpb.AccountingEntry {
	accountingEntries, err := suite.service.accountingRepository.FindBySource(ctx, orderId, collection)
	assert.NoError(suite.T(), err)

	return accountingEntries
}

func (suite *AccountingEntryTestSuite) helperCheckOrderView(orderId, orderCurrency, royaltyCurrency, vatCurrency string, orderControlResults map[string]float64) {
	orderView, err := suite.service.orderViewRepository.GetPrivateOrderBy(ctx, orderId, "", "")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), orderView)

	assert.Equal(suite.T(), orderView.PaymentGrossRevenueOrigin.Currency, orderCurrency)
	assert.Equal(suite.T(), orderView.PaymentGrossRevenue.Currency, royaltyCurrency)
	assert.Equal(suite.T(), orderView.PaymentGrossRevenueLocal.Currency, vatCurrency)

	assert.Equal(suite.T(), orderView.PaymentTaxFeeOrigin.Currency, orderCurrency)
	assert.Equal(suite.T(), orderView.PaymentTaxFee.Currency, royaltyCurrency)
	assert.Equal(suite.T(), orderView.PaymentTaxFeeLocal.Currency, vatCurrency)

	a := orderView.PaymentTaxFeeTotal.Amount
	b := orderControlResults["real_tax_fee"] + orderControlResults["central_bank_tax_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["real_tax_fee_total"])

	a = orderView.TaxFeeTotal.Amount
	b = orderControlResults["merchant_tax_fee_cost_value"] + orderControlResults["merchant_tax_fee_central_bank_fx"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["merchant_tax_fee"])

	a = orderView.FeesTotal.Amount
	b = orderControlResults["ps_method_fee"] + orderControlResults["merchant_ps_fixed_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))

	a = orderView.PaymentGrossRevenueFxProfit.Amount
	b = orderControlResults["ps_gross_revenue_fx"] - orderControlResults["ps_gross_revenue_fx_tax_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["ps_gross_revenue_fx_profit"])

	a = orderView.GrossRevenue.Amount
	b = orderControlResults["real_gross_revenue"] - orderControlResults["ps_gross_revenue_fx"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["merchant_gross_revenue"])

	a = orderView.PaysuperMethodFeeProfit.Amount
	b = orderControlResults["merchant_method_fee"] - orderControlResults["merchant_method_fee_cost_value"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["ps_markup_merchant_method_fee"])

	a = orderView.PaysuperMethodFixedFeeTariffFxProfit.Amount
	b = orderControlResults["merchant_method_fixed_fee"] - orderControlResults["real_merchant_method_fixed_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["markup_merchant_method_fixed_fee_fx"])

	a = orderView.PaysuperMethodFixedFeeTariffTotalProfit.Amount
	b = orderControlResults["real_merchant_method_fixed_fee"] - orderControlResults["real_merchant_method_fixed_fee_cost_value"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["ps_method_fixed_fee_profit"])

	a = orderView.PaysuperFixedFeeFxProfit.Amount
	b = orderControlResults["merchant_ps_fixed_fee"] - orderControlResults["real_merchant_ps_fixed_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["markup_merchant_ps_fixed_fee"])

	a = orderView.NetRevenue.Amount
	b = orderControlResults["real_gross_revenue"] -
		orderControlResults["merchant_tax_fee_central_bank_fx"] -
		orderControlResults["ps_gross_revenue_fx"] -
		orderControlResults["merchant_tax_fee_cost_value"] -
		orderControlResults["ps_method_fee"] -
		orderControlResults["merchant_ps_fixed_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["merchant_net_revenue"])

	a = orderView.PaysuperMethodTotalProfit.Amount
	b = orderControlResults["ps_method_fee"] +
		orderControlResults["merchant_ps_fixed_fee"] -
		orderControlResults["merchant_method_fee_cost_value"] -
		orderControlResults["real_merchant_method_fixed_fee_cost_value"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, orderControlResults["ps_method_profit"])

	a = orderView.PaysuperTotalProfit.Amount
	b = orderControlResults["ps_gross_revenue_fx"] +
		orderControlResults["ps_method_fee"] +
		orderControlResults["merchant_ps_fixed_fee"] -
		orderControlResults["central_bank_tax_fee"] -
		orderControlResults["ps_gross_revenue_fx_tax_fee"] -
		orderControlResults["merchant_method_fee_cost_value"] -
		orderControlResults["real_merchant_method_fixed_fee_cost_value"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))
	assert.Equal(suite.T(), a, tools.ToPrecise(orderControlResults["ps_profit_total"]))
}

func (suite *AccountingEntryTestSuite) helperCheckRefundView(orderId, orderCurrency, royaltyCurrency, vatCurrency string, refundControlResults map[string]float64) {
	orderView, err := suite.service.orderViewRepository.GetPrivateOrderBy(context.TODO(), orderId, "", "")
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), orderView.PaymentRefundGrossRevenueOrigin.Currency, orderCurrency)
	assert.Equal(suite.T(), orderView.PaymentRefundGrossRevenue.Currency, royaltyCurrency)
	assert.Equal(suite.T(), orderView.PaymentRefundGrossRevenueLocal.Currency, vatCurrency)

	assert.Equal(suite.T(), orderView.PaymentRefundTaxFeeOrigin.Currency, orderCurrency)
	assert.Equal(suite.T(), orderView.PaymentRefundTaxFee.Currency, royaltyCurrency)
	assert.Equal(suite.T(), orderView.PaymentRefundTaxFeeLocal.Currency, vatCurrency)

	a := orderView.RefundTaxFeeTotal.Amount
	b := refundControlResults["reverse_tax_fee"] + refundControlResults["reverse_tax_fee_delta"]
	assert.Equal(suite.T(), tools.ToPrecise(a), tools.ToPrecise(b))

	a = orderView.RefundFeesTotal.Amount
	b = refundControlResults["merchant_refund_fee"] + refundControlResults["merchant_refund_fixed_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))

	a = orderView.RefundGrossRevenueFx.Amount
	b = refundControlResults["merchant_refund"] - refundControlResults["real_refund"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))

	a = orderView.PaysuperMethodRefundFeeTariffProfit.Amount
	b = refundControlResults["merchant_refund_fee"] - refundControlResults["real_refund_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))

	a = orderView.PaysuperMethodRefundFixedFeeTariffProfit.Amount
	b = refundControlResults["merchant_refund_fixed_fee"] - refundControlResults["real_refund_fixed_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))

	a = orderView.RefundReverseRevenue.Amount
	b = refundControlResults["merchant_refund"] + refundControlResults["merchant_refund_fee"] + refundControlResults["merchant_refund_fixed_fee"] + refundControlResults["reverse_tax_fee_delta"] - refundControlResults["reverse_tax_fee"]
	assert.Equal(suite.T(), tools.ToPrecise(a), tools.ToPrecise(b))

	a = orderView.PaysuperRefundTotalProfit.Amount
	b = refundControlResults["merchant_refund_fee"] + refundControlResults["merchant_refund_fixed_fee"] + refundControlResults["ps_reverse_tax_fee_delta"] - refundControlResults["real_refund_fixed_fee"] - refundControlResults["real_refund_fee"]
	assert.Equal(suite.T(), a, tools.ToPrecise(b))

}

func (suite *AccountingEntryTestSuite) TestAccountingEntry_Ok_USD_EUR_None() {
	// Order currency USD
	// Royalty currency EUR
	// VAT currency NONE

	orderAmount := float64(650)
	orderCountry := "AO"
	orderCurrency := "USD"
	royaltyCurrency := "EUR"
	merchantCountry := "DE"

	merchant := HelperCreateMerchant(suite.Suite, suite.service, royaltyCurrency, merchantCountry, suite.paymentMethod, 0, suite.merchant.OperatingCompanyId)
	project := HelperCreateProject(suite.Suite, suite.service, merchant.Id, billingpb.VatPayerBuyer)

	country, err := suite.service.country.GetByIsoCodeA2(ctx, orderCountry)
	assert.NoError(suite.T(), err)

	paymentMerCost := &billingpb.PaymentChannelCostMerchant{
		MerchantId:              merchant.Id,
		Name:                    "MASTERCARD",
		PayoutCurrency:          royaltyCurrency,
		MinAmount:               0,
		Region:                  country.PayerTariffRegion,
		Country:                 country.IsoCodeA2,
		MethodPercent:           0.025,
		MethodFixAmount:         0.02,
		MethodFixAmountCurrency: "EUR",
		PsPercent:               0.05,
		PsFixedFee:              0.05,
		PsFixedFeeCurrency:      "EUR",
		MccCode:                 billingpb.MccCodeLowRisk,
	}

	err = suite.service.paymentChannelCostMerchantRepository.Insert(ctx, paymentMerCost)
	assert.NoError(suite.T(), err)

	order := HelperCreateAndPayOrder(suite.Suite, suite.service, orderAmount, orderCurrency, orderCountry, project, suite.paymentMethod, suite.cookie)
	assert.NotNil(suite.T(), order)

	orderAccountingEntries := suite.helperGetAccountingEntries(order.Id, repository.CollectionOrder)
	assert.Equal(suite.T(), len(orderAccountingEntries), 15)
}
