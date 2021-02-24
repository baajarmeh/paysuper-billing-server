package models

import (
	"bytes"
	"github.com/bxcodec/faker"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

type OrderTestSuite struct {
	suite.Suite
	mapper orderMapper
}

func TestOrderTestSuite(t *testing.T) {
	suite.Run(t, new(OrderTestSuite))
}

func (suite *OrderTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *OrderTestSuite) Test_Order_NewMapper() {
	mapper := NewOrderMapper()
	assert.IsType(suite.T(), &orderMapper{}, mapper)
}

func (suite *OrderTestSuite) Test_MapToMgo_Ok() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	original.Canceled = false
	original.PrivateStatus = 0
	original.Refunded = false
	original.Object = "order"
	original.BillingAddress.Country = "US"
	original.VirtualCurrencyAmount = 0
	original.Status = "created"
	original.TestingCase = ""
	original.CountryCode = "US"
	original.ReceiptEmail = original.User.Email
	original.ReceiptPhone = original.User.Phone
	original.Fee = nil
	original.NetRevenue = nil

	mgo, err := suite.mapper.MapObjectToMgo(original)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), mgo)

	obj, err := suite.mapper.MapMgoToObject(mgo)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), obj)

	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	marshaler := &jsonpb.Marshaler{}
	err = marshaler.Marshal(buf1, original)
	err = marshaler.Marshal(buf2, obj.(*billingpb.Order))

	assert.NoError(suite.T(), marshaler.Marshal(buf1, original))
	assert.NoError(suite.T(), marshaler.Marshal(buf2, obj.(*billingpb.Order)))
	assert.Equal(suite.T(), buf1.Bytes(), buf2.Bytes())
}

func (suite *OrderTestSuite) Test_Error_Items_CreatedAt() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	original.Items[0].CreatedAt = &timestamp.Timestamp{Seconds: -1, Nanos: -1}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *OrderTestSuite) Test_Error_Items_UpdatedAt() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)
	original.Items[0].UpdatedAt = &timestamp.Timestamp{Seconds: -1, Nanos: -1}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *OrderTestSuite) Test_Error_CreatedAt() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	original.CreatedAt = &timestamp.Timestamp{Seconds: -1, Nanos: -1}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *OrderTestSuite) Test_Error_RefundedAt() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	original.RefundedAt = &timestamp.Timestamp{Seconds: -1, Nanos: -1}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *OrderTestSuite) Test_Error_UpdatedAt() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	original.UpdatedAt = &timestamp.Timestamp{Seconds: -1, Nanos: -1}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *OrderTestSuite) Test_Error_ParentPaymentAt() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	original.ParentPaymentAt = &timestamp.Timestamp{Seconds: -1, Nanos: -1}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *OrderTestSuite) Test_Error_CancelledAt() {
	original := &billingpb.Order{}
	err := faker.FakeData(original)
	original.CanceledAt = &timestamp.Timestamp{Seconds: -1, Nanos: -1}
	_, err = suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}
