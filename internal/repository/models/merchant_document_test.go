package models

import (
	"bytes"
	"github.com/bxcodec/faker"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
)

type MerchantDocumentTestSuite struct {
	suite.Suite
	mapper merchantDocumentMapper
}

func TestMerchantDocumentTestSuite(t *testing.T) {
	suite.Run(t, new(MerchantDocumentTestSuite))
}

func (suite *MerchantDocumentTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *MerchantDocumentTestSuite) Test_MerchantDocument_NewMapper() {
	mapper := NewMerchantDocumentMapper()
	assert.IsType(suite.T(), &merchantDocumentMapper{}, mapper)
}

func (suite *MerchantDocumentTestSuite) Test_MapToMgo_Ok() {
	original := &billingpb.MerchantDocument{}
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
	assert.NoError(suite.T(), marshaler.Marshal(buf2, obj.(*billingpb.MerchantDocument)))
	assert.JSONEq(suite.T(), string(buf1.Bytes()), string(buf2.Bytes()))
}

func (suite *MerchantDocumentTestSuite) Test_Error_CreatedAt() {
	original := &billingpb.MerchantDocument{
		CreatedAt: &timestamp.Timestamp{Seconds: -1, Nanos: -1},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *MerchantDocumentTestSuite) Test_Error_MerchantId() {
	original := &billingpb.MerchantDocument{}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *MerchantDocumentTestSuite) Test_Error_UserId() {
	original := &billingpb.MerchantDocument{
		MerchantId: primitive.NewObjectID().Hex(),
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}
