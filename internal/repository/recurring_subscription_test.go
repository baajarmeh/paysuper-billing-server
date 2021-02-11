package repository

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type RecurringSubscriptionTestSuite struct {
	suite.Suite
	db         mongodb.SourceInterface
	repository *recurringSubscriptionRepository
	log        *zap.Logger
}

func Test_RecurringSubscription(t *testing.T) {
	suite.Run(t, new(RecurringSubscriptionTestSuite))
}

func (suite *RecurringSubscriptionTestSuite) SetupTest() {
	_, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

	suite.log, err = zap.NewProduction()
	assert.NoError(suite.T(), err, "Logger initialization failed")

	suite.db, err = mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	suite.repository = &recurringSubscriptionRepository{db: suite.db, mapper: models.NewRecurringSubscriptionMapper()}
}

func (suite *RecurringSubscriptionTestSuite) TearDownTest() {
	if err := suite.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	if err := suite.db.Close(); err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *RecurringSubscriptionTestSuite) TestRecurringSubscription_NewRecurringSubscriptionRepository_Ok() {
	repository := NewRecurringSubscriptionRepository(suite.db)
	assert.IsType(suite.T(), &recurringSubscriptionRepository{}, repository)
}

func (suite *RecurringSubscriptionTestSuite) TestRecurringSubscription_Insert() {
	subscription := suite.template()

	err := suite.repository.Insert(context.TODO(), subscription)
	assert.NoError(suite.T(), err)

	subscription2, err := suite.repository.GetById(context.TODO(), subscription.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), subscription.Id, subscription2.Id)
	assert.NotEmpty(suite.T(), subscription2.Plan)
	assert.Equal(suite.T(), subscription.Plan.Id, subscription2.Plan.Id)
	assert.Equal(suite.T(), subscription.Customer, subscription2.Customer)
	assert.Equal(suite.T(), subscription.Project, subscription2.Project)
	assert.Equal(suite.T(), subscription.Status, subscription2.Status)
	assert.Equal(suite.T(), subscription.TotalAmount, subscription2.TotalAmount)
	assert.Equal(suite.T(), subscription.ItemType, subscription2.ItemType)
	assert.Equal(suite.T(), subscription.ItemList, subscription2.ItemList)
	assert.Equal(suite.T(), subscription.CardpayPlanId, subscription2.CardpayPlanId)
	assert.Equal(suite.T(), subscription.CardpaySubscriptionId, subscription2.CardpaySubscriptionId)
	assert.Equal(suite.T(), subscription.ExpireAt.Seconds, subscription2.ExpireAt.Seconds)
	assert.Equal(suite.T(), subscription.LastPaymentAt.Seconds, subscription2.LastPaymentAt.Seconds)
	assert.NotEmpty(suite.T(), subscription2.CreatedAt)
	assert.NotEmpty(suite.T(), subscription2.UpdatedAt)
}

func (suite *RecurringSubscriptionTestSuite) TestRecurringSubscription_Update() {
	subscription := suite.template()

	err := suite.repository.Insert(context.TODO(), subscription)
	assert.NoError(suite.T(), err)

	subscription2, err := suite.repository.GetById(context.TODO(), subscription.Id)
	assert.NoError(suite.T(), err)

	subscription2.Plan = &billingpb.RecurringPlan{
		Id:          primitive.NewObjectID().Hex(),
		MerchantId:  primitive.NewObjectID().Hex(),
		ProjectId:   primitive.NewObjectID().Hex(),
		Status:      billingpb.RecurringSubscriptionStatusActive,
		Name:        map[string]string{"ru": "ru"},
		Description: map[string]string{"en": "en"},
		ExternalId:  "ext_id2",
		Tags:        []string{"tag2"},
		GroupId:     "group_id2",
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Type:  billingpb.RecurringPeriodMinute,
				Value: 100,
			},
			Currency: "USD",
			Amount:   11,
		},
		Expiration: &billingpb.RecurringPlanPeriod{
			Type:  billingpb.RecurringPeriodDay,
			Value: 20,
		},
		Trial: &billingpb.RecurringPlanPeriod{
			Type:  billingpb.RecurringPeriodMonth,
			Value: 30,
		},
		GracePeriod: &billingpb.RecurringPlanPeriod{
			Type:  billingpb.RecurringPeriodWeek,
			Value: 40,
		},
	}
	subscription2.Customer = &billingpb.RecurringSubscriptionCustomer{
		Id:         primitive.NewObjectID().Hex(),
		Uuid:       "uuid2",
		ExternalId: "external_id2",
		Email:      "email2",
		Phone:      "phone2",
	}
	subscription2.Project = &billingpb.RecurringSubscriptionProject{
		Id:   primitive.NewObjectID().Hex(),
		Name: map[string]string{"ru": "text"},
	}
	subscription2.Status = billingpb.RecurringSubscriptionStatusCanceled
	subscription2.ItemType = pkg.OrderType_product
	subscription2.ItemList = []string{primitive.NewObjectID().Hex()}
	subscription2.CardpayPlanId = "cp_plan_id2"
	subscription2.CardpaySubscriptionId = "cp_subscription_id2"
	subscription2.TotalAmount = 20
	subscription2.ExpireAt = ptypes.TimestampNow()
	subscription2.LastPaymentAt = ptypes.TimestampNow()

	err = suite.repository.Update(context.TODO(), subscription2)
	assert.NoError(suite.T(), err)

	subscription3, err := suite.repository.GetById(context.TODO(), subscription.Id)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), subscription.Id, subscription3.Id)
	assert.NotEmpty(suite.T(), subscription3.Plan)
	assert.Equal(suite.T(), subscription2.Plan.Id, subscription3.Plan.Id)
	assert.Equal(suite.T(), subscription2.Customer, subscription3.Customer)
	assert.Equal(suite.T(), subscription2.Project, subscription3.Project)
	assert.Equal(suite.T(), subscription2.Status, subscription3.Status)
	assert.Equal(suite.T(), subscription2.TotalAmount, subscription3.TotalAmount)
	assert.Equal(suite.T(), subscription2.ItemType, subscription3.ItemType)
	assert.Equal(suite.T(), subscription2.ItemList, subscription3.ItemList)
	assert.Equal(suite.T(), subscription2.CardpayPlanId, subscription3.CardpayPlanId)
	assert.Equal(suite.T(), subscription2.CardpaySubscriptionId, subscription3.CardpaySubscriptionId)
	assert.Equal(suite.T(), subscription2.ExpireAt.Seconds, subscription3.ExpireAt.Seconds)
	assert.Equal(suite.T(), subscription2.LastPaymentAt.Seconds, subscription3.LastPaymentAt.Seconds)
	assert.Equal(suite.T(), subscription2.CreatedAt, subscription3.CreatedAt)
	assert.NotEmpty(suite.T(), subscription3.UpdatedAt)
}

func (suite *RecurringSubscriptionTestSuite) TestGetByPlanIdCustomerId() {
	subscription1 := suite.template()
	err := suite.repository.Insert(context.TODO(), subscription1)
	assert.NoError(suite.T(), err)

	subscription2 := suite.template()
	err = suite.repository.Insert(context.TODO(), subscription2)
	assert.NoError(suite.T(), err)

	subscription3, err := suite.repository.GetByPlanIdCustomerId(context.TODO(), subscription1.Plan.Id, subscription1.Customer.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), subscription3)
	assert.Equal(suite.T(), subscription1.Id, subscription3.Id)

	subscription4, err := suite.repository.GetByPlanIdCustomerId(context.TODO(), subscription2.Plan.Id, subscription2.Customer.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), subscription4)
	assert.Equal(suite.T(), subscription2.Id, subscription4.Id)
}

func (suite *RecurringSubscriptionTestSuite) TestGetActiveByPlanIdCustomerId() {
	subscription1 := suite.template()
	err := suite.repository.Insert(context.TODO(), subscription1)
	assert.NoError(suite.T(), err)

	subscription2 := suite.template()
	subscription2.Status = billingpb.RecurringSubscriptionStatusCanceled
	err = suite.repository.Insert(context.TODO(), subscription2)
	assert.NoError(suite.T(), err)

	subscription3, err := suite.repository.GetActiveByPlanIdCustomerId(context.TODO(), subscription1.Plan.Id, subscription1.Customer.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), subscription3)
	assert.Equal(suite.T(), subscription1.Id, subscription3.Id)

	subscription4, err := suite.repository.GetActiveByPlanIdCustomerId(context.TODO(), subscription2.Plan.Id, subscription2.Customer.Id)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), subscription4)
}

func (suite *RecurringSubscriptionTestSuite) TestFindByCustomerId() {
	subscription1 := suite.template()
	err := suite.repository.Insert(context.TODO(), subscription1)
	assert.NoError(suite.T(), err)

	subscription2 := suite.template()
	err = suite.repository.Insert(context.TODO(), subscription2)
	assert.NoError(suite.T(), err)

	subscriptions1, err := suite.repository.FindByCustomerId(context.TODO(), subscription1.Customer.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), subscriptions1)
	assert.Len(suite.T(), subscriptions1, 1)
	assert.Equal(suite.T(), subscription1.Id, subscriptions1[0].Id)

	subscriptions2, err := suite.repository.FindByCustomerId(context.TODO(), subscription2.Customer.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), subscriptions2)
	assert.Len(suite.T(), subscriptions2, 1)
	assert.Equal(suite.T(), subscription2.Id, subscriptions2[0].Id)
}

func (suite *RecurringSubscriptionTestSuite) TestFindByMerchantIdCustomerId() {
	subscription1 := suite.template()
	err := suite.repository.Insert(context.TODO(), subscription1)
	assert.NoError(suite.T(), err)

	subscription2 := suite.template()
	err = suite.repository.Insert(context.TODO(), subscription2)
	assert.NoError(suite.T(), err)

	subscriptions1, err := suite.repository.FindByMerchantIdCustomerId(context.TODO(), subscription1.Plan.MerchantId, subscription1.Customer.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), subscriptions1)
	assert.Len(suite.T(), subscriptions1, 1)
	assert.Equal(suite.T(), subscription1.Id, subscriptions1[0].Id)

	subscriptions2, err := suite.repository.FindByMerchantIdCustomerId(context.TODO(), subscription2.Plan.MerchantId, subscription2.Customer.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), subscriptions2)
	assert.Len(suite.T(), subscriptions2, 1)
	assert.Equal(suite.T(), subscription2.Id, subscriptions2[0].Id)
}

func (suite *RecurringSubscriptionTestSuite) template() *billingpb.RecurringSubscription {
	return &billingpb.RecurringSubscription{
		Id: primitive.NewObjectID().Hex(),
		Plan: &billingpb.RecurringPlan{
			Id:          primitive.NewObjectID().Hex(),
			MerchantId:  primitive.NewObjectID().Hex(),
			ProjectId:   primitive.NewObjectID().Hex(),
			Status:      billingpb.RecurringSubscriptionStatusActive,
			Name:        map[string]string{"en": "en"},
			Description: map[string]string{"ru": "ru"},
			ExternalId:  "ext_id",
			Tags:        []string{"tag"},
			GroupId:     "group_id",
			Charge: &billingpb.RecurringPlanCharge{
				Period: &billingpb.RecurringPlanPeriod{
					Type:  billingpb.RecurringPeriodMonth,
					Value: 1,
				},
				Currency: "RUB",
				Amount:   100,
			},
			Expiration: &billingpb.RecurringPlanPeriod{
				Type:  billingpb.RecurringPeriodYear,
				Value: 2,
			},
			Trial: &billingpb.RecurringPlanPeriod{
				Type:  billingpb.RecurringPeriodWeek,
				Value: 3,
			},
			GracePeriod: &billingpb.RecurringPlanPeriod{
				Type:  billingpb.RecurringPeriodDay,
				Value: 4,
			},
		},
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id:         primitive.NewObjectID().Hex(),
			Uuid:       "uuid",
			ExternalId: "external_id",
			Email:      "email",
			Phone:      "phone",
		},
		Project: &billingpb.RecurringSubscriptionProject{
			Id:   primitive.NewObjectID().Hex(),
			Name: map[string]string{"en": "name"},
		},
		Status:                billingpb.RecurringSubscriptionStatusActive,
		ItemType:              pkg.OrderType_simple,
		ItemList:              []string{primitive.NewObjectID().Hex()},
		CardpayPlanId:         "cp_plan_id",
		CardpaySubscriptionId: "cp_subscription_id",
		TotalAmount:           10,
		ExpireAt:              ptypes.TimestampNow(),
		LastPaymentAt:         ptypes.TimestampNow(),
		CreatedAt:             ptypes.TimestampNow(),
		UpdatedAt:             ptypes.TimestampNow(),
	}
}
