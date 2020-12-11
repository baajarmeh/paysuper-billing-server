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
	"time"
)

type RoyaltyReportTestSuite struct {
	suite.Suite
	mapper royaltyReportMapper
}

func TestRoyaltyReportTestSuite(t *testing.T) {
	suite.Run(t, new(RoyaltyReportTestSuite))
}

func (suite *RoyaltyReportTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_NewRoyaltyReportMapper() {
	mapper := NewRoyaltyReportMapper()
	assert.IsType(suite.T(), &royaltyReportMapper{}, mapper)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_Ok() {
	original := &billingpb.RoyaltyReport{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	original.MerchantName = ""

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
	assert.NoError(suite.T(), marshaler.Marshal(buf2, obj.(*billingpb.RoyaltyReport)))
	assert.JSONEq(suite.T(), string(buf1.Bytes()), string(buf2.Bytes()))
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_Error_Id() {
	original := &billingpb.RoyaltyReport{
		Id:         "test",
		MerchantId: primitive.NewObjectID().Hex(),
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_Error_MerchantId() {
	original := &billingpb.RoyaltyReport{
		MerchantId: "id",
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_Error_CreatedAt() {
	original := &billingpb.RoyaltyReport{
		CreatedAt:  &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		MerchantId: primitive.NewObjectID().Hex(),
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_Error_UpdatedAt() {
	original := &billingpb.RoyaltyReport{
		CreatedAt:  nil,
		UpdatedAt:  &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		MerchantId: primitive.NewObjectID().Hex(),
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_Ok_EmptyUpdatedAt() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.NoError(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorPeriodFrom() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorPeriodTo() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorAcceptExpireAt() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorPayoutDate() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorAcceptedAt() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorDisputeStartedAt() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: -1, Nanos: -1},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorDisputeClosedAt() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: -1, Nanos: -1},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorSummeryCorrection() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		Summary: &billingpb.RoyaltyReportSummary{
			Corrections: []*billingpb.RoyaltyReportCorrectionItem{{
				EntryDate: &timestamp.Timestamp{Seconds: -1, Nanos: -1},
			}},
		},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapRoyaltyReportToMgo_ErrorSummeryRollingReserves() {
	original := &billingpb.RoyaltyReport{
		MerchantId:       primitive.NewObjectID().Hex(),
		PeriodFrom:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PeriodTo:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptExpireAt:   &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		PayoutDate:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		AcceptedAt:       &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeStartedAt: &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		DisputeClosedAt:  &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
		Summary: &billingpb.RoyaltyReportSummary{
			RollingReserves: []*billingpb.RoyaltyReportCorrectionItem{{
				EntryDate: &timestamp.Timestamp{Seconds: -1, Nanos: -1},
			}},
		},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Ok() {
	original := &MgoRoyaltyReport{}
	original.Merchants = nil
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	obj, err := suite.mapper.MapMgoToObject(original)
	assert.NoError(suite.T(), err)

	mgo, err := suite.mapper.MapObjectToMgo(obj)
	assert.NoError(suite.T(), err)

	assert.ObjectsAreEqualValues(original, mgo)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_CreatedAt() {
	original := &MgoRoyaltyReport{
		CreatedAt: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_UpdatedAt() {
	original := &MgoRoyaltyReport{
		CreatedAt: time.Now(),
		UpdatedAt: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_PayoutDate() {
	original := &MgoRoyaltyReport{
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		PayoutDate: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_PeriodFrom() {
	original := &MgoRoyaltyReport{
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		PayoutDate: time.Now(),
		PeriodFrom: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_PeriodTo() {
	original := &MgoRoyaltyReport{
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		PayoutDate: time.Now(),
		PeriodFrom: time.Now(),
		PeriodTo:   time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_AcceptExpireAt() {
	original := &MgoRoyaltyReport{
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		PayoutDate:     time.Now(),
		PeriodFrom:     time.Now(),
		PeriodTo:       time.Now(),
		AcceptExpireAt: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_AcceptedAt() {
	original := &MgoRoyaltyReport{
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		PayoutDate:     time.Now(),
		PeriodFrom:     time.Now(),
		PeriodTo:       time.Now(),
		AcceptExpireAt: time.Now(),
		AcceptedAt:     time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_DisputeStartedAt() {
	original := &MgoRoyaltyReport{
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		PayoutDate:       time.Now(),
		PeriodFrom:       time.Now(),
		PeriodTo:         time.Now(),
		AcceptExpireAt:   time.Now(),
		AcceptedAt:       time.Now(),
		DisputeStartedAt: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_DisputeClosedAt() {
	original := &MgoRoyaltyReport{
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		PayoutDate:       time.Now(),
		PeriodFrom:       time.Now(),
		PeriodTo:         time.Now(),
		AcceptExpireAt:   time.Now(),
		AcceptedAt:       time.Now(),
		DisputeStartedAt: time.Now(),
		DisputeClosedAt:  time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_SummaryCorrections() {
	original := &MgoRoyaltyReport{
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		PayoutDate:       time.Now(),
		PeriodFrom:       time.Now(),
		PeriodTo:         time.Now(),
		AcceptExpireAt:   time.Now(),
		AcceptedAt:       time.Now(),
		DisputeStartedAt: time.Now(),
		DisputeClosedAt:  time.Now(),
		Summary: &MgoRoyaltyReportSummary{
			Corrections: []*MgoRoyaltyReportCorrectionItem{{
				EntryDate: time.Time{}.AddDate(-10000, 0, 0),
			}},
		},
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportTestSuite) Test_RoyaltyReport_MapMgoToRoyaltyReport_Error_SummaryRollingReserves() {
	original := &MgoRoyaltyReport{
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		PayoutDate:       time.Now(),
		PeriodFrom:       time.Now(),
		PeriodTo:         time.Now(),
		AcceptExpireAt:   time.Now(),
		AcceptedAt:       time.Now(),
		DisputeStartedAt: time.Now(),
		DisputeClosedAt:  time.Now(),
		Summary: &MgoRoyaltyReportSummary{
			RollingReserves: []*MgoRoyaltyReportCorrectionItem{{
				EntryDate: time.Time{}.AddDate(-10000, 0, 0),
			}},
		},
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

type RoyaltyReportCorrectionItemTestSuite struct {
	suite.Suite
	mapper royaltyReportCorrectionItemMapper
}

func TestRoyaltyReportCorrectionItemTestSuite(t *testing.T) {
	suite.Run(t, new(RoyaltyReportCorrectionItemTestSuite))
}

func (suite *RoyaltyReportCorrectionItemTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *RoyaltyReportCorrectionItemTestSuite) Test_RoyaltyReport_NewRoyaltyReportCorrectionItemMapper() {
	mapper := NewRoyaltyReportCorrectionItemMapper()
	assert.IsType(suite.T(), &royaltyReportCorrectionItemMapper{}, mapper)
}

func (suite *RoyaltyReportCorrectionItemTestSuite) Test_RoyaltyReportCorrectionItem_MapRoyaltyReportToMgo_Ok() {
	original := &billingpb.RoyaltyReportCorrectionItem{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	mgo, err := suite.mapper.MapObjectToMgo(original)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), mgo)

	obj, err := suite.mapper.MapMgoToObject(mgo)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), obj)

	assert.ObjectsAreEqualValues(original, obj)
}

func (suite *RoyaltyReportCorrectionItemTestSuite) Test_RoyaltyReportCorrectionItem_MapRoyaltyReportToMgo_Error_AccountingEntryId() {
	original := &billingpb.RoyaltyReportCorrectionItem{
		AccountingEntryId: "test",
		EntryDate:         &timestamp.Timestamp{Seconds: 1582890052, Nanos: 0},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportCorrectionItemTestSuite) Test_RoyaltyReportCorrectionItem_MapRoyaltyReportToMgo_Error_EntryDate() {
	original := &billingpb.RoyaltyReportCorrectionItem{
		AccountingEntryId: primitive.NewObjectID().Hex(),
		EntryDate:         &timestamp.Timestamp{Seconds: -1, Nanos: -1},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportCorrectionItemTestSuite) Test_RoyaltyReportCorrectionItem_MapMgoToRoyaltyReport_Ok() {
	original := &MgoRoyaltyReportCorrectionItem{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	obj, err := suite.mapper.MapMgoToObject(original)
	assert.NoError(suite.T(), err)

	mgo, err := suite.mapper.MapObjectToMgo(obj)
	assert.NoError(suite.T(), err)

	assert.ObjectsAreEqualValues(original, mgo)
}

func (suite *RoyaltyReportCorrectionItemTestSuite) Test_RoyaltyReportCorrectionItem_MapMgoToRoyaltyReport_Error_DisputeStartedAt() {
	original := &MgoRoyaltyReportCorrectionItem{
		EntryDate: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}

type RoyaltyReportChangesTestSuite struct {
	suite.Suite
	mapper royaltyReportChangesMapper
}

func TestRoyaltyReportChangesTestSuite(t *testing.T) {
	suite.Run(t, new(RoyaltyReportChangesTestSuite))
}

func (suite *RoyaltyReportChangesTestSuite) SetupTest() {
	InitFakeCustomProviders()
}

func (suite *RoyaltyReportChangesTestSuite) Test_RoyaltyReport_NewRoyaltyReportCorrectionItemMapper() {
	mapper := NewRoyaltyReportChangesMapper()
	assert.IsType(suite.T(), &royaltyReportChangesMapper{}, mapper)
}

func (suite *RoyaltyReportChangesTestSuite) Test_RoyaltyReportChanges_MapRoyaltyReportToMgo_Ok() {
	original := &billingpb.RoyaltyReportChanges{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	mgo, err := suite.mapper.MapObjectToMgo(original)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), mgo)

	obj, err := suite.mapper.MapMgoToObject(mgo)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), obj)

	assert.ObjectsAreEqualValues(original, obj)
}

func (suite *RoyaltyReportChangesTestSuite) Test_RoyaltyReportChanges_MapRoyaltyReportToMgo_Error_Id() {
	original := &billingpb.RoyaltyReportChanges{
		Id: "test",
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportChangesTestSuite) Test_RoyaltyReportChanges_MapRoyaltyReportToMgo_Error_CreatedAt() {
	original := &billingpb.RoyaltyReportChanges{
		CreatedAt: &timestamp.Timestamp{Seconds: -1, Nanos: -1},
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportChangesTestSuite) Test_RoyaltyReportChanges_MapRoyaltyReportToMgo_Error_RoyaltyReportId() {
	original := &billingpb.RoyaltyReportChanges{
		RoyaltyReportId: "test",
	}
	_, err := suite.mapper.MapObjectToMgo(original)
	assert.Error(suite.T(), err)
}

func (suite *RoyaltyReportChangesTestSuite) Test_RoyaltyReportChanges_MapMgoToRoyaltyReport_Ok() {
	original := &MgoRoyaltyReportChanges{}
	err := faker.FakeData(original)
	assert.NoError(suite.T(), err)

	obj, err := suite.mapper.MapMgoToObject(original)
	assert.NoError(suite.T(), err)

	mgo, err := suite.mapper.MapObjectToMgo(obj)
	assert.NoError(suite.T(), err)

	assert.ObjectsAreEqualValues(original, mgo)
}

func (suite *RoyaltyReportChangesTestSuite) Test_RoyaltyReportChanges_MapMgoToRoyaltyReport_Error_DisputeStartedAt() {
	original := &MgoRoyaltyReportChanges{
		CreatedAt: time.Time{}.AddDate(-10000, 0, 0),
	}
	_, err := suite.mapper.MapMgoToObject(original)
	assert.Error(suite.T(), err)
}
