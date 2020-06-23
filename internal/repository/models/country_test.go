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

type CountryTestSuite struct {
	suite.Suite
	mapper countryMapper
}

func TestCountryTestSuite(t *testing.T) {
	suite.Run(t, new(CountryTestSuite))
}

func (suite *CountryTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *CountryTestSuite) Test_NewMapper() {
	mapper := NewCountryMapper()
	assert.IsType(suite.T(), &countryMapper{}, mapper)
}

func (suite *CountryTestSuite) Test_MapToMgo_Ok() {
	original := &billingpb.Country{}
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
	assert.NoError(suite.T(), marshaler.Marshal(buf2, obj.(*billingpb.Country)))
	assert.JSONEq(suite.T(), string(buf1.Bytes()), string(buf2.Bytes()))
}

func (suite *CountryTestSuite) Test_Error_CreatedAt() {
	original := &billingpb.Country{
		CreatedAt: &timestamp.Timestamp{Seconds: -1, Nanos: -1},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *CountryTestSuite) Test_Error_ReservedTo() {
	original := &billingpb.Country{
		UpdatedAt: &timestamp.Timestamp{Seconds: -1, Nanos: -1},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}
