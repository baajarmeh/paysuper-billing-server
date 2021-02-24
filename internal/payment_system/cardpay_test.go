package payment_system

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"io/ioutil"
	"testing"
)

var (
	bankCardRequisites = map[string]string{
		billingpb.PaymentCreateFieldPan:    "4000000000000002",
		billingpb.PaymentCreateFieldMonth:  "12",
		billingpb.PaymentCreateFieldYear:   "2019",
		billingpb.PaymentCreateFieldHolder: "Mr. Card Holder",
		billingpb.PaymentCreateFieldCvv:    "000",
	}

	orderSimpleBankCard = &billingpb.Order{
		Id: primitive.NewObjectID().Hex(),
		Project: &billingpb.ProjectOrder{
			Id:         primitive.NewObjectID().Hex(),
			Name:       map[string]string{"en": "Project Name"},
			UrlSuccess: "http://localhost/success",
			UrlFail:    "http://localhost/false",
		},
		Description:        fmt.Sprintf(pkg.OrderDefaultDescription, primitive.NewObjectID().Hex()),
		PrivateStatus:      recurringpb.OrderStatusNew,
		CreatedAt:          ptypes.TimestampNow(),
		IsJsonRequest:      false,
		Items:              []*billingpb.OrderItem{},
		TotalPaymentAmount: 10.2,
		Currency:           "RUB",
		User: &billingpb.OrderUser{
			Id:     primitive.NewObjectID().Hex(),
			Object: "user",
			Email:  "test@unit.test",
			Ip:     "127.0.0.1",
			Locale: "ru",
			Address: &billingpb.OrderBillingAddress{
				Country:    "RU",
				City:       "St.Petersburg",
				PostalCode: "190000",
				State:      "SPE",
			},
			TechEmail: fmt.Sprintf("%s@paysuper.com", primitive.NewObjectID().Hex()),
		},
		PaymentMethod: &billingpb.PaymentMethodOrder{
			Id:         primitive.NewObjectID().Hex(),
			Name:       "Bank card",
			Handler:    "cardpay",
			ExternalId: "BANKCARD",
			Params: &billingpb.PaymentMethodParams{
				Currency:       "USD",
				TerminalId:     "123456",
				Secret:         "secret_key",
				SecretCallback: "callback_secret_key",
				ApiUrl:         "https://sandbox.cardpay.com",
			},
			PaymentSystemId: primitive.NewObjectID().Hex(),
			Group:           "BANKCARD",
			Saved:           false,
		},
	}
)

type CardPayTestSuite struct {
	suite.Suite

	cfg          *config.Config
	handler      PaymentSystemInterface
	typedHandler *cardPay
	logObserver  *zap.Logger
	zapRecorder  *observer.ObservedLogs
}

func Test_CardPay(t *testing.T) {
	suite.Run(t, new(CardPayTestSuite))
}

func (suite *CardPayTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	suite.cfg = cfg

	var core zapcore.Core

	lvl := zap.NewAtomicLevel()
	core, suite.zapRecorder = observer.New(lvl)
	suite.logObserver = zap.New(core)
	zap.ReplaceGlobals(suite.logObserver)

	suite.handler = NewCardPayHandler()
	handler, ok := suite.handler.(*cardPay)
	assert.True(suite.T(), ok)
	suite.typedHandler = handler
}

func (suite *CardPayTestSuite) TearDownTest() {}

func (suite *CardPayTestSuite) TestCardPay_GetCardPayOrder_Ok() {
	res, err := suite.typedHandler.getCardPayOrder(
		orderSimpleBankCard,
		suite.cfg.GetRedirectUrlSuccess(nil),
		suite.cfg.GetRedirectUrlFail(nil),
		bankCardRequisites,
	)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), res.CardAccount)
	assert.NotNil(suite.T(), res.ReturnUrls)
	assert.Equal(suite.T(), res.ReturnUrls.SuccessUrl, suite.cfg.GetRedirectUrlSuccess(nil))
	assert.Equal(suite.T(), res.ReturnUrls.CancelUrl, suite.cfg.GetRedirectUrlFail(nil))
	assert.Equal(suite.T(), res.ReturnUrls.DeclineUrl, suite.cfg.GetRedirectUrlFail(nil))
}

func (suite *CardPayTestSuite) TestCardPay_CreatePayment_Mock_Ok() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	url, err := suite.handler.CreatePayment(
		orderSimpleBankCard,
		suite.cfg.GetRedirectUrlSuccess(nil),
		suite.cfg.GetRedirectUrlFail(nil),
		bankCardRequisites,
	)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), url)
}

func (suite *CardPayTestSuite) TestCardPay_getRequestWithAuth_WithData_Ok() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	order := &billingpb.Order{PaymentMethod: &billingpb.PaymentMethodOrder{Params: &billingpb.PaymentMethodParams{
		ApiUrl:     "http://127.0.0.1",
		TerminalId: "test",
	}}}
	data := &CardPayRecurringPlan{Id: "planId"}
	req, err := suite.typedHandler.getRequestWithAuth(order, data, pkg.PaymentSystemActionCreatePayment)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), req.Body)

	body := &CardPayRecurringPlan{}
	b, err := ioutil.ReadAll(req.Body)
	assert.NoError(suite.T(), err)

	err = json.Unmarshal(b, &body)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), data.Id, body.Id)
}

func (suite *CardPayTestSuite) TestCardPay_getRequestWithAuth_WithUrlParams_Ok() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	order := &billingpb.Order{PaymentMethod: &billingpb.PaymentMethodOrder{Params: &billingpb.PaymentMethodParams{
		ApiUrl:     "http://127.0.0.1",
		TerminalId: "test",
	}}}
	action := pkg.PaymentSystemActionDeleteRecurringPlan
	planId := "id"
	req, err := suite.typedHandler.getRequestWithAuth(order, nil, action, planId)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf(pkg.CardPayPaths[action].Path, planId), req.URL.Path)
}

func (suite *CardPayTestSuite) TestCardPay_getRequestWithAuth_InvalidAction() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	order := &billingpb.Order{PaymentMethod: &billingpb.PaymentMethodOrder{Params: &billingpb.PaymentMethodParams{
		ApiUrl:     "http://127.0.0.1",
		TerminalId: "test",
	}}}
	_, err := suite.typedHandler.getRequestWithAuth(order, nil, "action")
	assert.Error(suite.T(), err)
}

func (suite *CardPayTestSuite) TestCardPay_getRequestWithAuth_InvalidApiUrl() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	order := &billingpb.Order{PaymentMethod: &billingpb.PaymentMethodOrder{Params: &billingpb.PaymentMethodParams{
		ApiUrl:     "",
		TerminalId: "test",
	}}}
	_, err := suite.typedHandler.getRequestWithAuth(order, nil, pkg.PaymentSystemActionDeleteRecurringPlan)
	assert.Error(suite.T(), err)
}

func (suite *CardPayTestSuite) TestCardPay_getRequestWithAuth_CheckAuthHeader_Ok() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	order := &billingpb.Order{PaymentMethod: &billingpb.PaymentMethodOrder{Params: &billingpb.PaymentMethodParams{
		ApiUrl:     "http://127.0.0.1",
		TerminalId: "test",
	}}}
	req, err := suite.typedHandler.getRequestWithAuth(order, nil, pkg.PaymentSystemActionDeleteRecurringPlan)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), req.Header.Get(pkg.HeaderAuthorization))
}

func (suite *CardPayTestSuite) TestCardPay_IsSubscriptionCallback_True() {
	req := &billingpb.CardPayPaymentCallback{RecurringData: &billingpb.CardPayCallbackRecurringData{
		Subscription: &billingpb.CardPayCallbackRecurringDataSubscription{Id: "id"},
	}}
	assert.True(suite.T(), suite.handler.IsSubscriptionCallback(req))
}

func (suite *CardPayTestSuite) TestCardPay_IsSubscriptionCallback_False() {
	req := &billingpb.CardPayPaymentCallback{RecurringData: &billingpb.CardPayCallbackRecurringData{}}
	assert.False(suite.T(), suite.handler.IsSubscriptionCallback(req))
}

func (suite *CardPayTestSuite) TestCardPay_CreateRecurringSubscription_Ok() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	suite.typedHandler.httpClient.Transport = &TransportCardPayRecurringPlanOk{}

	order := orderSimpleBankCard
	subscription := &billingpb.RecurringSubscription{
		Plan: &billingpb.RecurringPlan{
			Charge: &billingpb.RecurringPlanCharge{
				Period: &billingpb.RecurringPlanPeriod{
					Type:  billingpb.RecurringPeriodDay,
					Value: 1,
				},
				Amount:   10,
				Currency: "EUR",
			},
		},
	}

	url, err := suite.handler.CreateRecurringSubscription(
		order,
		subscription,
		suite.cfg.GetRedirectUrlSuccess(nil),
		suite.cfg.GetRedirectUrlFail(nil),
		bankCardRequisites,
	)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), url)
	assert.NotEmpty(suite.T(), subscription.CardpayPlanId)
	assert.NotEmpty(suite.T(), subscription.CardpaySubscriptionId)
}

func (suite *CardPayTestSuite) TestCardPay_CreateRecurringSubscription_InactivePlan() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()
	suite.typedHandler.httpClient.Transport = &TransportCardPayRecurringPlanInactive{}

	order := orderSimpleBankCard
	subscription := &billingpb.RecurringSubscription{
		Plan: &billingpb.RecurringPlan{
			Charge: &billingpb.RecurringPlanCharge{
				Period: &billingpb.RecurringPlanPeriod{
					Type:  billingpb.RecurringPeriodDay,
					Value: 1,
				},
				Amount:   10,
				Currency: "EUR",
			},
		},
	}

	_, err := suite.handler.CreateRecurringSubscription(
		order,
		subscription,
		suite.cfg.GetRedirectUrlSuccess(nil),
		suite.cfg.GetRedirectUrlFail(nil),
		bankCardRequisites,
	)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), paymentSystemErrorCreateRecurringPlanFailed, err)
}

func (suite *CardPayTestSuite) TestCardPay_DeleteRecurringSubscription_Ok() {
	suite.typedHandler.httpClient = NewCardPayHttpClientStatusOk()

	subscription := &billingpb.RecurringSubscription{
		CardpayPlanId:         "planId",
		CardpaySubscriptionId: "subscriptionId",
	}

	err := suite.handler.DeleteRecurringSubscription(
		orderSimpleBankCard,
		subscription,
	)
	assert.NoError(suite.T(), err)
}
