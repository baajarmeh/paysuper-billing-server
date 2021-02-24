package service

import (
	"context"
	"github.com/go-redis/redis"
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
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"net"
	"testing"
)

type TokenTestSuite struct {
	suite.Suite
	service *Service
	cache   database.CacheInterface

	project                            *billingpb.Project
	projectWithProducts                *billingpb.Project
	projectWithVirtualCurrencyProducts *billingpb.Project
	projectWithMerchantWithoutTariffs  *billingpb.Project

	product1    *billingpb.Product
	product2    *billingpb.Product
	keyProducts []*billingpb.KeyProduct
	product3    *billingpb.Product

	defaultTokenReq *billingpb.TokenRequest
}

func Test_Token(t *testing.T) {
	suite.Run(t, new(TokenTestSuite))
}

func (suite *TokenTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

	db, err := mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	paymentMinLimitSystem1 := &billingpb.PaymentMinLimitSystem{
		Id:        primitive.NewObjectID().Hex(),
		Currency:  "RUB",
		Amount:    0.01,
		CreatedAt: ptypes.TimestampNow(),
		UpdatedAt: ptypes.TimestampNow(),
	}

	pgRub := &billingpb.PriceGroup{
		Id:       primitive.NewObjectID().Hex(),
		Region:   "RUB",
		Currency: "RUB",
		IsActive: true,
	}
	pgUsd := &billingpb.PriceGroup{
		Id:       primitive.NewObjectID().Hex(),
		Region:   "USD",
		Currency: "USD",
		IsActive: true,
	}
	ru := &billingpb.Country{
		IsoCodeA2:       "RU",
		Region:          "Russia",
		Currency:        "RUB",
		PaymentsAllowed: true,
		ChangeAllowed:   true,
		VatEnabled:      true,
		PriceGroupId:    pgRub.Id,
		VatCurrency:     "RUB",
		VatThreshold: &billingpb.CountryVatThreshold{
			Year:  0,
			World: 0,
		},
		VatPeriodMonth:         3,
		VatDeadlineDays:        25,
		VatStoreYears:          5,
		VatCurrencyRatesPolicy: "last-day",
		VatCurrencyRatesSource: "cbrf",
	}
	us := &billingpb.Country{
		IsoCodeA2:       "US",
		Region:          "USD",
		Currency:        "USD",
		PaymentsAllowed: true,
		ChangeAllowed:   true,
		VatEnabled:      true,
		PriceGroupId:    pgRub.Id,
		VatCurrency:     "USD",
		VatThreshold: &billingpb.CountryVatThreshold{
			Year:  0,
			World: 0,
		},
		VatPeriodMonth:         3,
		VatDeadlineDays:        25,
		VatStoreYears:          5,
		VatCurrencyRatesPolicy: "last-day",
		VatCurrencyRatesSource: "cbrf",
	}

	merchant := &billingpb.Merchant{
		Id: primitive.NewObjectID().Hex(),
		Company: &billingpb.MerchantCompanyInfo{
			Name:               "Unit test",
			AlternativeName:    "merchant1",
			Website:            "http://localhost",
			Country:            "RU",
			Zip:                "190000",
			City:               "St.Petersburg",
			Address:            "address",
			AddressAdditional:  "address_additional",
			RegistrationNumber: "registration_number",
		},
		Contacts: &billingpb.MerchantContact{
			Authorized: &billingpb.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "123456789",
				Position: "Unit Test",
			},
			Technical: &billingpb.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "123456789",
			},
		},
		Banking: &billingpb.MerchantBanking{
			Currency:             "RUB",
			Name:                 "Bank name",
			Address:              "address",
			AccountNumber:        "0000001",
			Swift:                "swift",
			CorrespondentAccount: "correspondent_account",
			Details:              "details",
		},
		IsVatEnabled:              true,
		IsCommissionToUserEnabled: true,
		Status:                    billingpb.MerchantStatusAgreementSigned,
		IsSigned:                  true,
		Tariff: &billingpb.MerchantTariff{
			Payment: []*billingpb.MerchantTariffRatesPayment{
				{
					MinAmount:              0,
					MaxAmount:              4.99,
					MethodName:             "VISA",
					MethodPercentFee:       1.8,
					MethodFixedFee:         0.2,
					MethodFixedFeeCurrency: "USD",
					PsPercentFee:           3.0,
					PsFixedFee:             0.3,
					PsFixedFeeCurrency:     "USD",
					MerchantHomeRegion:     "russia_and_cis",
					PayerRegion:            "europe",
				},
				{
					MinAmount:              5,
					MaxAmount:              999999999.99,
					MethodName:             "MasterCard",
					MethodPercentFee:       1.8,
					MethodFixedFee:         0.2,
					MethodFixedFeeCurrency: "USD",
					PsPercentFee:           3.0,
					PsFixedFee:             0.3,
					PsFixedFeeCurrency:     "USD",
					MerchantHomeRegion:     "russia_and_cis",
					PayerRegion:            "europe",
				},
			},
			Payout: &billingpb.MerchantTariffRatesSettingsItem{
				MethodPercentFee:       0,
				MethodFixedFee:         25.0,
				MethodFixedFeeCurrency: "EUR",
				IsPaidByMerchant:       true,
			},
			HomeRegion: "russia_and_cis",
		},
	}
	merchantWithoutTariffs := &billingpb.Merchant{
		Id: primitive.NewObjectID().Hex(),
		Company: &billingpb.MerchantCompanyInfo{
			Name:               "Unit test",
			AlternativeName:    "merchant1",
			Website:            "http://localhost",
			Country:            "RU",
			Zip:                "190000",
			City:               "St.Petersburg",
			Address:            "address",
			AddressAdditional:  "address_additional",
			RegistrationNumber: "registration_number",
		},
		Contacts: &billingpb.MerchantContact{
			Authorized: &billingpb.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "123456789",
				Position: "Unit Test",
			},
			Technical: &billingpb.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "123456789",
			},
		},
		Banking: &billingpb.MerchantBanking{
			Currency:             "RUB",
			Name:                 "Bank name",
			Address:              "address",
			AccountNumber:        "0000001",
			Swift:                "swift",
			CorrespondentAccount: "correspondent_account",
			Details:              "details",
		},
		IsVatEnabled:              true,
		IsCommissionToUserEnabled: true,
		Status:                    billingpb.MerchantStatusAgreementSigned,
		IsSigned:                  true,
	}

	project := &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         billingpb.ProjectCallbackProtocolEmpty,
		LimitsCurrency:           "RUB",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       false,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   billingpb.ProjectStatusInProduction,
		MerchantId:               merchant.Id,
	}
	projectWithProducts := &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         billingpb.ProjectCallbackProtocolEmpty,
		LimitsCurrency:           "RUB",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       true,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   billingpb.ProjectStatusInProduction,
		MerchantId:               merchant.Id,
	}
	projectWithMerchantWithoutTariffs := &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         billingpb.ProjectCallbackProtocolEmpty,
		LimitsCurrency:           "RUB",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       true,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   billingpb.ProjectStatusInProduction,
		MerchantId:               merchantWithoutTariffs.Id,
	}
	projectWithVirtualCurrencyProducts := &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         billingpb.ProjectCallbackProtocolEmpty,
		LimitsCurrency:           "RUB",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       true,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   billingpb.ProjectStatusInProduction,
		MerchantId:               merchant.Id,
		VirtualCurrency: &billingpb.ProjectVirtualCurrency{
			Name: map[string]string{"en": "test project 1"},
			Prices: []*billingpb.ProductPrice{
				{Amount: 100, Currency: "RUB", Region: "RUB"},
				{Amount: 10, Currency: "USD", Region: "USD"},
			},
		},
	}

	product3 := &billingpb.Product{
		Id:              primitive.NewObjectID().Hex(),
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_double_yeti",
		Name:            map[string]string{"en": initialName},
		DefaultCurrency: "RUB",
		Enabled:         true,
		Description:     map[string]string{"en": "blah-blah-blah"},
		LongDescription: map[string]string{"en": "Super game steam keys"},
		Url:             "http://test.ru/dffdsfsfs",
		Images:          []string{"/home/image.jpg"},
		MerchantId:      projectWithVirtualCurrencyProducts.MerchantId,
		ProjectId:       projectWithVirtualCurrencyProducts.Id,
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
		Prices: []*billingpb.ProductPrice{{Amount: 10.00, IsVirtualCurrency: true}},
	}

	product1 := &billingpb.Product{
		Id:              primitive.NewObjectID().Hex(),
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_double_yeti",
		Name:            map[string]string{"en": initialName},
		DefaultCurrency: "RUB",
		Enabled:         true,
		Description:     map[string]string{"en": "blah-blah-blah"},
		LongDescription: map[string]string{"en": "Super game steam keys"},
		Url:             "http://test.ru/dffdsfsfs",
		Images:          []string{"/home/image.jpg"},
		MerchantId:      projectWithProducts.MerchantId,
		ProjectId:       projectWithProducts.Id,
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
		Prices: []*billingpb.ProductPrice{{Currency: "RUB", Amount: 1005.00, Region: "RUB"}},
	}
	product2 := &billingpb.Product{
		Id:              primitive.NewObjectID().Hex(),
		Object:          "product1",
		Type:            "simple_product",
		Sku:             "ru_double_yeti1",
		Name:            map[string]string{"en": initialName},
		DefaultCurrency: "RUB",
		Enabled:         true,
		Description:     map[string]string{"en": "blah-blah-blah"},
		LongDescription: map[string]string{"en": "Super game steam keys"},
		Url:             "http://test.ru/dffdsfsfs",
		Images:          []string{"/home/image.jpg"},
		MerchantId:      projectWithProducts.MerchantId,
		ProjectId:       projectWithProducts.Id,
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
		Prices: []*billingpb.ProductPrice{{Currency: "RUB", Amount: 1005.00, Region: "RUB"}},
	}

	redisClient := database.NewRedis(
		&redis.Options{
			Addr:     cfg.RedisHost,
			Password: cfg.RedisPassword,
		},
	)

	redisdb := mocks.NewTestRedis()
	suite.cache, err = database.NewCacheRedis(redisdb, "cache")

	if err != nil {
		suite.FailNow("Cache redis initialize failed", "%v", err)
	}

	suite.service = NewBillingService(
		db,
		cfg,
		mocks.NewGeoIpServiceTestOk(),
		nil,
		nil,
		nil,
		redisClient,
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

	limits := []*billingpb.PaymentMinLimitSystem{paymentMinLimitSystem1}
	err = suite.service.paymentMinLimitSystemRepository.MultipleInsert(context.TODO(), limits)
	assert.NoError(suite.T(), err)

	err = suite.service.merchantRepository.MultipleInsert(context.TODO(), []*billingpb.Merchant{merchant, merchantWithoutTariffs})

	if err != nil {
		suite.FailNow("Insert merchant test data failed", "%v", err)
	}

	projects := []*billingpb.Project{project, projectWithProducts, projectWithMerchantWithoutTariffs, projectWithVirtualCurrencyProducts}
	err = suite.service.project.MultipleInsert(context.TODO(), projects)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
	}

	err = suite.service.country.MultipleInsert(context.TODO(), []*billingpb.Country{ru, us})

	if err != nil {
		suite.FailNow("Insert country test data failed", "%v", err)
	}

	err = suite.service.priceGroupRepository.MultipleInsert(context.TODO(), []*billingpb.PriceGroup{pgRub, pgUsd})

	if err != nil {
		suite.FailNow("Insert price group test data failed", "%v", err)
	}

	suite.project = project
	suite.projectWithProducts = projectWithProducts
	suite.projectWithVirtualCurrencyProducts = projectWithVirtualCurrencyProducts
	suite.projectWithMerchantWithoutTariffs = projectWithMerchantWithoutTariffs
	suite.product1 = product1
	suite.product2 = product2
	suite.product3 = product3

	suite.keyProducts = CreateKeyProductsForProject(suite.Suite, suite.service, suite.project, 3)

	err = suite.service.productRepository.MultipleInsert(context.TODO(), []*billingpb.Product{product1, product2, product3})
	assert.NoError(suite.T(), err, "Insert product test data failed")

	suite.defaultTokenReq = &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Email: &billingpb.TokenUserEmailValue{
				Value:    "test@unit.test",
				Verified: true,
			},
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
			Ip: &billingpb.TokenUserIpValue{
				Value: "127.0.0.1",
			},
			UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.89 Safari/537.36",
			AcceptLanguage: "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:     suite.project.Id,
			Amount:        100,
			Currency:      "RUB",
			Type:          pkg.OrderType_simple,
			ButtonCaption: "unit test",
		},
	}
}

func (suite *TokenTestSuite) TearDownTest() {
	err := suite.service.db.Drop()

	if err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	err = suite.service.db.Close()

	if err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *TokenTestSuite) TestToken_CreateToken_NewCustomer_Ok() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Email: &billingpb.TokenUserEmailValue{
				Value:    "test@unit.test",
				Verified: true,
			},
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
			Ip: &billingpb.TokenUserIpValue{
				Value: "127.0.0.1",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:     suite.project.Id,
			Amount:        100,
			Currency:      "RUB",
			Type:          pkg.OrderType_simple,
			ButtonCaption: "unit test",
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	rep := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)

	customer, err := suite.service.customerRepository.GetById(context.TODO(), rep.token.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)

	assert.Equal(suite.T(), req.User.Id, customer.ExternalId)
	assert.Equal(suite.T(), customer.Id+pkg.TechEmailDomain, customer.TechEmail)
	assert.Equal(suite.T(), req.User.Email.Value, customer.Email)
	assert.Equal(suite.T(), req.User.Email.Verified, customer.EmailVerified)
	assert.Empty(suite.T(), customer.Phone)
	assert.False(suite.T(), customer.PhoneVerified)
	assert.Empty(suite.T(), customer.Name)
	assert.NotEmpty(suite.T(), customer.Ip)
	assert.Equal(suite.T(), req.User.Ip.Value, net.IP(customer.Ip).String())
	assert.Equal(suite.T(), req.User.Locale.Value, customer.Locale)
	assert.Empty(suite.T(), customer.AcceptLanguage)
	assert.Empty(suite.T(), customer.UserAgent)
	assert.Len(suite.T(), customer.IpHistory, 1)
	assert.Equal(suite.T(), req.User.Ip.Value, customer.IpHistory[0].IpString)
	assert.EqualValues(suite.T(), net.ParseIP(req.User.Ip.Value), customer.IpHistory[0].Ip)
	assert.NotNil(suite.T(), customer.IpHistory[0].Address)
	assert.Equal(suite.T(), "RU", customer.IpHistory[0].Address.Country)
	assert.Equal(suite.T(), "St.Petersburg", customer.IpHistory[0].Address.City)
	assert.Equal(suite.T(), "190000", customer.IpHistory[0].Address.PostalCode)
	assert.Equal(suite.T(), "SPE", customer.IpHistory[0].Address.State)
	assert.NotNil(suite.T(), customer.Address)
	assert.Equal(suite.T(), "RU", customer.Address.Country)
	assert.Equal(suite.T(), "St.Petersburg", customer.Address.City)
	assert.Equal(suite.T(), "190000", customer.Address.PostalCode)
	assert.Equal(suite.T(), "SPE", customer.Address.State)
	assert.Len(suite.T(), customer.AddressHistory, 1)
	assert.NotNil(suite.T(), customer.Address)
	assert.Equal(suite.T(), "RU", customer.AddressHistory[0].Country)
	assert.Equal(suite.T(), "St.Petersburg", customer.AddressHistory[0].City)
	assert.Equal(suite.T(), "190000", customer.AddressHistory[0].PostalCode)
	assert.Equal(suite.T(), "SPE", customer.AddressHistory[0].State)
	assert.Empty(suite.T(), customer.AcceptLanguageHistory)
	assert.Empty(suite.T(), customer.Metadata)
	assert.NotEmpty(suite.T(), customer.Uuid)

	assert.Len(suite.T(), customer.Identity, 2)
	assert.Equal(suite.T(), customer.Identity[0].Value, customer.ExternalId)
	assert.True(suite.T(), customer.Identity[0].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeExternal, customer.Identity[0].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[0].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[0].MerchantId)

	assert.Equal(suite.T(), customer.Identity[1].Value, customer.Email)
	assert.True(suite.T(), customer.Identity[1].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customer.Identity[1].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[1].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[1].MerchantId)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ExistCustomer_Ok() {
	email := "test_exist_customer@unit.test"

	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Email: &billingpb.TokenUserEmailValue{
				Value:    email,
				Verified: true,
			},
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    100,
			Currency:  "RUB",
			Type:      pkg.OrderType_simple,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	req.User.Phone = &billingpb.TokenUserPhoneValue{
		Value: "1234567890",
	}
	req.User.Email = &billingpb.TokenUserEmailValue{
		Value: "test_exist_customer_1@unit.test",
	}
	rsp1 := &billingpb.TokenResponse{}
	err = suite.service.CreateToken(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotEmpty(suite.T(), rsp1.Token)

	rep := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)

	rep1 := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep1.getToken(rsp1.Token)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), rep.token.CustomerId, rep1.token.CustomerId)

	customer, err := suite.service.customerRepository.GetById(context.TODO(), rep.token.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), customer)

	assert.Len(suite.T(), customer.Identity, 4)
	assert.Equal(suite.T(), customer.Identity[3].Value, customer.Phone)
	assert.False(suite.T(), customer.Identity[3].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypePhone, customer.Identity[3].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[3].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[3].MerchantId)

	assert.Equal(suite.T(), customer.Identity[2].Value, customer.Email)
	assert.False(suite.T(), customer.Identity[2].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customer.Identity[2].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[2].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[2].MerchantId)

	assert.Equal(suite.T(), email, customer.Identity[1].Value)
	assert.True(suite.T(), customer.Identity[1].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customer.Identity[1].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[1].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[1].MerchantId)

	assert.Equal(suite.T(), customer.Identity[0].Value, customer.ExternalId)
	assert.True(suite.T(), customer.Identity[0].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeExternal, customer.Identity[0].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[0].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[0].MerchantId)

	assert.NotEmpty(suite.T(), customer.Uuid)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ExistCustomer_UpdateExistIdentity_Ok() {
	email := "test_exist_customer_update_exist_identity@unit.test"
	address := &billingpb.OrderBillingAddress{
		Country:    "UA",
		City:       "NewYork",
		PostalCode: "000000",
	}

	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Email: &billingpb.TokenUserEmailValue{
				Value: email,
			},
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
			Address: address,
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    100,
			Currency:  "RUB",
			Type:      pkg.OrderType_simple,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	rep := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)

	customer, err := suite.service.customerRepository.GetById(context.TODO(), rep.token.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)
	assert.NotEmpty(suite.T(), customer.Identity)
	assert.False(suite.T(), customer.Identity[1].Verified)

	req.User.Phone = &billingpb.TokenUserPhoneValue{
		Value: "1234567890",
	}
	req.User.Email = &billingpb.TokenUserEmailValue{
		Value:    "test_exist_customer_update_exist_identity@unit.test",
		Verified: true,
	}
	req.User.Name = &billingpb.TokenUserValue{Value: "Unit test"}
	req.User.Ip = &billingpb.TokenUserIpValue{Value: "127.0.0.1"}
	req.User.Locale = &billingpb.TokenUserLocaleValue{Value: "ru"}
	req.User.Address = &billingpb.OrderBillingAddress{
		Country:    "RU",
		City:       "St.Petersburg",
		PostalCode: "190000",
	}
	rsp1 := &billingpb.TokenResponse{}
	err = suite.service.CreateToken(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotEmpty(suite.T(), rsp1.Token)

	rep = &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)

	rep1 := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep1.getToken(rsp.Token)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), rep.token.CustomerId, rep1.token.CustomerId)

	customer, err = suite.service.customerRepository.GetById(context.TODO(), rep.token.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), customer)

	assert.Len(suite.T(), customer.Identity, 3)
	assert.Equal(suite.T(), customer.Identity[2].Value, customer.Phone)
	assert.False(suite.T(), customer.Identity[2].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypePhone, customer.Identity[2].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[2].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[2].MerchantId)

	assert.Equal(suite.T(), email, customer.Identity[1].Value)
	assert.True(suite.T(), customer.Identity[1].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customer.Identity[1].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[1].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[1].MerchantId)

	assert.Equal(suite.T(), customer.Identity[0].Value, customer.ExternalId)
	assert.True(suite.T(), customer.Identity[0].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeExternal, customer.Identity[0].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[0].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[0].MerchantId)

	assert.Equal(suite.T(), req.User.Name.Value, customer.Name)
	assert.Equal(suite.T(), req.User.Ip.Value, net.IP(customer.Ip).String())
	assert.Equal(suite.T(), req.User.Locale.Value, customer.Locale)
	assert.Equal(suite.T(), req.User.Address, customer.Address)

	assert.Len(suite.T(), customer.IpHistory, 1)
	assert.NotEmpty(suite.T(), customer.LocaleHistory)
	assert.NotEmpty(suite.T(), customer.AddressHistory)

	assert.Equal(suite.T(), address.Country, customer.AddressHistory[0].Country)
	assert.Equal(suite.T(), address.City, customer.AddressHistory[0].City)
	assert.Equal(suite.T(), address.PostalCode, customer.AddressHistory[0].PostalCode)
}

func (suite *TokenTestSuite) TestToken_CreateToken_CustomerIdentityInformationNotFound_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    100,
			Currency:  "RUB",
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorUserIdentityRequired, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectNotFound_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: primitive.NewObjectID().Hex(),
			Amount:    100,
			Currency:  "RUB",
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorProjectNotFound, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_AmountIncorrect_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    -100,
			Currency:  "RUB",
			Type:      pkg.OrderType_simple,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsSimpleCheckoutParamsRequired, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_ProductInvalid_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.projectWithProducts.Id,
			ProductsIds: []string{primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex()},
			Type:        pkg.OrderType_product,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorProductsInvalid, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_Ok() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.projectWithProducts.Id,
			ProductsIds: []string{suite.product1.Id, suite.product2.Id},
			Type:        pkg.OrderType_product,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	rep := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)

	assert.Len(suite.T(), rep.token.Settings.ProductsIds, 2)
}

func (suite *TokenTestSuite) TestToken_CreateToken_MerchantWithoutTariffs_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.projectWithMerchantWithoutTariffs.Id,
			Amount:    100,
			Currency:  "USD",
			Type:      pkg.OrderType_simple,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorMerchantBadTariffs, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProductCheckout_ProductListEmpty_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.project.Id,
			Type:      pkg.OrderType_product,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsProductAndKeyProductIdsParamsRequired, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_SimpleCheckout_InvalidCurrency_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    1000,
			Currency:  "KZT",
			Type:      pkg.OrderType_simple,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorCurrencyNotFound, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_SimpleCheckout_LimitsAmount_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    0.1,
			Currency:  "RUB",
			Type:      pkg.OrderType_simple,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorAmountLowerThanMinAllowed, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_KeyProductCheckout_Ok() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.project.Id,
			ProductsIds: []string{suite.keyProducts[0].Id, suite.keyProducts[1].Id},
			PlatformId:  "steam",
			Type:        pkg.OrderType_key,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_KeyProductCheckout_ProjectWithoutKeyProducts_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.projectWithProducts.Id,
			ProductsIds: []string{suite.keyProducts[0].Id, suite.keyProducts[1].Id},
			PlatformId:  "steam",
			Type:        pkg.OrderType_key,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorProductsInvalid, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_UnknownType_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId: suite.projectWithProducts.Id,
			Type:      "unknown",
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsTypeRequired, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_KeyProductCheckout_WithAmount_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.project.Id,
			ProductsIds: []string{suite.keyProducts[0].Id, suite.keyProducts[1].Id},
			Type:        pkg.OrderType_key,
			Amount:      100,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsAmountAndCurrencyParamNotAllowedForType, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_KeyProductCheckout_WithCurrency_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.project.Id,
			ProductsIds: []string{suite.keyProducts[0].Id, suite.keyProducts[1].Id},
			Type:        pkg.OrderType_key,
			Currency:    "RUB",
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsAmountAndCurrencyParamNotAllowedForType, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_KeyProductCheckout_WithAmountAndCurrency_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.project.Id,
			ProductsIds: []string{suite.keyProducts[0].Id, suite.keyProducts[1].Id},
			Type:        pkg.OrderType_key,
			Amount:      100,
			Currency:    "RUB",
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsAmountAndCurrencyParamNotAllowedForType, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_SimpleCheckout_WithProductIds_Error() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Email: &billingpb.TokenUserEmailValue{
				Value:    "test@unit.test",
				Verified: true,
			},
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:   suite.project.Id,
			Amount:      100,
			Currency:    "RUB",
			Type:        pkg.OrderType_simple,
			ProductsIds: []string{suite.keyProducts[0].Id, suite.keyProducts[1].Id},
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsProductIdsParamNotAllowedForType, rsp.Message)
	assert.Empty(suite.T(), rsp.Token)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectWithVirtualCurrency_Ok() {
	req := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Id: primitive.NewObjectID().Hex(),
			Locale: &billingpb.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billingpb.TokenSettings{
			ProjectId:               suite.projectWithVirtualCurrencyProducts.Id,
			ProductsIds:             []string{suite.product3.Id},
			Type:                    pkg.OrderType_product,
			IsBuyForVirtualCurrency: true,
		},
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	rep := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)

	assert.Len(suite.T(), rep.token.Settings.ProductsIds, 1)
	assert.True(suite.T(), rep.token.Settings.IsBuyForVirtualCurrency)
}

func (suite *TokenTestSuite) TestToken_CreateToken_Customer_AddressReplaceGeoAddress_Ok() {
	suite.defaultTokenReq.User.Address = &billingpb.OrderBillingAddress{
		Country:    "US",
		State:      "AK",
		City:       "Metlakatla",
		PostalCode: "99926",
	}
	rsp := &billingpb.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), suite.defaultTokenReq, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	rep := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)

	customer, err := suite.service.customerRepository.GetById(context.TODO(), rep.token.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)

	assert.Len(suite.T(), customer.IpHistory, 1)
	assert.Equal(suite.T(), suite.defaultTokenReq.User.Ip.Value, customer.IpHistory[0].IpString)
	assert.EqualValues(suite.T(), net.ParseIP(suite.defaultTokenReq.User.Ip.Value), customer.IpHistory[0].Ip)
	assert.NotNil(suite.T(), customer.IpHistory[0].Address)
	assert.Equal(suite.T(), "RU", customer.IpHistory[0].Address.Country)
	assert.Equal(suite.T(), "St.Petersburg", customer.IpHistory[0].Address.City)
	assert.Equal(suite.T(), "190000", customer.IpHistory[0].Address.PostalCode)
	assert.Equal(suite.T(), "SPE", customer.IpHistory[0].Address.State)
	assert.NotNil(suite.T(), customer.Address)
	assert.Equal(suite.T(), "US", customer.Address.Country)
	assert.Equal(suite.T(), "Metlakatla", customer.Address.City)
	assert.Equal(suite.T(), "99926", customer.Address.PostalCode)
	assert.Equal(suite.T(), "AK", customer.Address.State)
	assert.Len(suite.T(), customer.AddressHistory, 1)
	assert.NotNil(suite.T(), customer.Address)
	assert.Equal(suite.T(), "US", customer.AddressHistory[0].Country)
	assert.Equal(suite.T(), "Metlakatla", customer.AddressHistory[0].City)
	assert.Equal(suite.T(), "99926", customer.AddressHistory[0].PostalCode)
	assert.Equal(suite.T(), "AK", customer.AddressHistory[0].State)

	suite.defaultTokenReq.User.Ip.Value = "127.0.0.2"
	suite.defaultTokenReq.User.Address = nil
	err = suite.service.CreateToken(context.TODO(), suite.defaultTokenReq, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	customer, err = suite.service.customerRepository.GetById(context.TODO(), rep.token.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)

	assert.Len(suite.T(), customer.IpHistory, 2)
	assert.Equal(suite.T(), "127.0.0.1", customer.IpHistory[0].IpString)
	assert.EqualValues(suite.T(), net.ParseIP("127.0.0.1"), customer.IpHistory[0].Ip)
	assert.Equal(suite.T(), "127.0.0.2", customer.IpHistory[1].IpString)
	assert.EqualValues(suite.T(), net.ParseIP("127.0.0.2"), customer.IpHistory[1].Ip)

	assert.NotNil(suite.T(), customer.IpHistory[0].Address)
	assert.Equal(suite.T(), "RU", customer.IpHistory[0].Address.Country)
	assert.Equal(suite.T(), "St.Petersburg", customer.IpHistory[0].Address.City)
	assert.Equal(suite.T(), "190000", customer.IpHistory[0].Address.PostalCode)
	assert.Equal(suite.T(), "SPE", customer.IpHistory[0].Address.State)
	assert.NotNil(suite.T(), customer.IpHistory[1].Address)
	assert.Equal(suite.T(), "US", customer.IpHistory[1].Address.Country)
	assert.Equal(suite.T(), "New York", customer.IpHistory[1].Address.City)
	assert.Equal(suite.T(), "14905", customer.IpHistory[1].Address.PostalCode)
	assert.Equal(suite.T(), "NY", customer.IpHistory[1].Address.State)

	assert.NotNil(suite.T(), customer.Address)
	assert.Equal(suite.T(), "US", customer.Address.Country)
	assert.Equal(suite.T(), "New York", customer.Address.City)
	assert.Equal(suite.T(), "14905", customer.Address.PostalCode)
	assert.Equal(suite.T(), "NY", customer.Address.State)

	assert.Len(suite.T(), customer.AddressHistory, 2)
	assert.Equal(suite.T(), "US", customer.AddressHistory[0].Country)
	assert.Equal(suite.T(), "Metlakatla", customer.AddressHistory[0].City)
	assert.Equal(suite.T(), "99926", customer.AddressHistory[0].PostalCode)
	assert.Equal(suite.T(), "AK", customer.AddressHistory[0].State)
	assert.Equal(suite.T(), "US", customer.AddressHistory[1].Country)
	assert.Equal(suite.T(), "New York", customer.AddressHistory[1].City)
	assert.Equal(suite.T(), "14905", customer.AddressHistory[1].PostalCode)
	assert.Equal(suite.T(), "NY", customer.AddressHistory[1].State)

	err = suite.service.CreateToken(context.TODO(), suite.defaultTokenReq, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	customer, err = suite.service.customerRepository.GetById(context.TODO(), rep.token.CustomerId)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), customer)

	assert.Len(suite.T(), customer.IpHistory, 2)
	assert.Len(suite.T(), customer.AddressHistory, 2)
}

func (suite *TokenTestSuite) TestToken_CreateToken_WithRecurringPlan() {
	recurringPlan := &billingpb.RecurringPlan{
		Id:         primitive.NewObjectID().Hex(),
		MerchantId: suite.project.MerchantId,
		ProjectId:  suite.project.Id,
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Type:  billingpb.RecurringPeriodDay,
				Value: 1,
			},
			Currency: "RUB",
			Amount:   1,
		},
		Status: pkg.RecurringPlanStatusActive,
		Name:   map[string]string{"en": "name"},
	}
	err := suite.service.recurringPlanRepository.Insert(context.TODO(), recurringPlan)
	assert.NoError(suite.T(), err)

	suite.defaultTokenReq.Settings.RecurringPlanId = recurringPlan.Id
	rsp := &billingpb.TokenResponse{}
	err = suite.service.CreateToken(context.TODO(), suite.defaultTokenReq, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), billingpb.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotEmpty(suite.T(), rsp.Token)

	rep := &tokenRepository{
		service: suite.service,
		token:   &Token{},
	}
	err = rep.getToken(rsp.Token)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), suite.defaultTokenReq.Settings.RecurringPlanId, rep.token.Settings.RecurringPlanId)
}
