package models

import (
	"bytes"
	"github.com/bxcodec/faker"
	"github.com/golang/protobuf/jsonpb"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
)

type RecurringPlanTestSuite struct {
	suite.Suite
	mapper recurringPlanMapper
}

func TestRecurringPlanTestSuite(t *testing.T) {
	suite.Run(t, new(RecurringPlanTestSuite))
}

func (suite *RecurringPlanTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *RecurringPlanTestSuite) Test_NewRecurringPlanMapper() {
	mapper := NewRecurringPlanMapper()
	assert.IsType(suite.T(), &recurringPlanMapper{}, mapper)
}

func (suite *RecurringPlanTestSuite) Test_MapObjectToMgo_Ok() {
	original := &billingpb.RecurringPlan{}
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
	assert.NoError(suite.T(), marshaler.Marshal(buf2, obj.(*billingpb.RecurringPlan)))

	assert.JSONEq(suite.T(), string(buf1.Bytes()), string(buf2.Bytes()))
}

func (suite *RecurringPlanTestSuite) Test_MapObjectToMgo_ErrorId() {
	original := &billingpb.RecurringPlan{
		Id: "test",
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RecurringPlanTestSuite) Test_MapObjectToMgo_ErrorMerchantId() {
	original := &billingpb.RecurringPlan{
		MerchantId: "test",
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RecurringPlanTestSuite) Test_MapObjectToMgo_ErrorProjectId() {
	original := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  "test",
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RecurringPlanTestSuite) Test_MapObjectToMgo_ErrorEmptyCharge() {
	original := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge:     nil,
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RecurringPlanTestSuite) Test_MapObjectToMgo_ErrorEmptyChargePeriod() {
	original := &billingpb.RecurringPlan{
		MerchantId: primitive.NewObjectID().Hex(),
		ProjectId:  primitive.NewObjectID().Hex(),
		Charge: &billingpb.RecurringPlanCharge{
			Period: nil,
		},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}
