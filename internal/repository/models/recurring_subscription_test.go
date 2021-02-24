package models

import (
	"bytes"
	"github.com/bxcodec/faker"
	"github.com/golang/protobuf/jsonpb"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

type RecurringSubscriptionTestSuite struct {
	suite.Suite
	mapper recurringSubscriptionMapper
}

func TestRecurringSubscriptionTestSuite(t *testing.T) {
	suite.Run(t, new(RecurringSubscriptionTestSuite))
}

func (suite *RecurringSubscriptionTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *RecurringSubscriptionTestSuite) Test_NewRecurringSubscriptionMapper() {
	mapper := NewRecurringSubscriptionMapper()
	assert.IsType(suite.T(), &recurringSubscriptionMapper{}, mapper)
}

func (suite *RecurringSubscriptionTestSuite) Test_MapObjectToMgo_Ok() {
	original := &billingpb.RecurringSubscription{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	mgo, err := suite.mapper.MapObjectToMgo(original)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), mgo)

	obj, err := suite.mapper.MapMgoToObject(mgo)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), obj)

	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	marshaler := &jsonpb.Marshaler{}

	assert.NoError(suite.T(), marshaler.Marshal(buf1, original))
	assert.NoError(suite.T(), marshaler.Marshal(buf2, obj.(*billingpb.RecurringSubscription)))

	assert.JSONEq(suite.T(), string(buf1.Bytes()), string(buf2.Bytes()))
}

func (suite *RecurringSubscriptionTestSuite) Test_MapObjectToMgo_ErrorId() {
	original := &billingpb.RecurringSubscription{
		Id: "test",
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RecurringSubscriptionTestSuite) Test_MapObjectToMgo_ErrorPlanId() {
	original := &billingpb.RecurringSubscription{
		Plan: &billingpb.RecurringPlan{
			Id: "test",
		},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RecurringSubscriptionTestSuite) Test_MapObjectToMgo_ErrorCustomerId() {
	plan := &billingpb.RecurringPlan{}
	err := faker.FakeData(plan)
	assert.NoError(suite.T(), err)

	original := &billingpb.RecurringSubscription{
		Plan: plan,
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id: "test",
		},
	}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RecurringSubscriptionTestSuite) Test_MapObjectToMgo_ErrorProjectId() {
	plan := &billingpb.RecurringPlan{}
	err := faker.FakeData(plan)
	assert.NoError(suite.T(), err)

	customer := &billingpb.RecurringSubscriptionCustomer{}
	err = faker.FakeData(customer)
	assert.NoError(suite.T(), err)

	original := &billingpb.RecurringSubscription{
		Plan:     plan,
		Customer: customer,
		Project: &billingpb.RecurringSubscriptionProject{
			Id: "test",
		},
	}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}
