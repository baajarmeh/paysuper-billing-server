package repository

import (
	"context"
	"errors"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/mocks"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	mongodbMocks "gopkg.in/paysuper/paysuper-database-mongo.v2/mocks"
	"testing"
)

type MerchantDocumentTestSuite struct {
	suite.Suite
	db         mongodb.SourceInterface
	repository *merchantDocumentRepository
	log        *zap.Logger
}

func Test_MerchantDocument(t *testing.T) {
	suite.Run(t, new(MerchantDocumentTestSuite))
}

func (suite *MerchantDocumentTestSuite) SetupTest() {
	_, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

	suite.log, err = zap.NewProduction()
	assert.NoError(suite.T(), err, "Logger initialization failed")

	suite.db, err = mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	suite.repository = &merchantDocumentRepository{db: suite.db, cache: &mocks.CacheInterface{}, mapper: models.NewMerchantDocumentMapper()}
}

func (suite *MerchantDocumentTestSuite) TearDownTest() {
	if err := suite.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	if err := suite.db.Close(); err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_NewMerchantDocumentRepository_Ok() {
	repository := NewMerchantDocumentRepository(suite.db)
	assert.IsType(suite.T(), &merchantDocumentRepository{}, repository)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_Insert_MapError() {
	document := suite.getMerchantDocumentTemplate()

	mapper := &mocks.Mapper{}
	mapper.On("MapMgoToObject", mock.Anything).Return(nil, errors.New("error"))
	mapper.On("MapObjectToMgo", mock.Anything).Return(nil, errors.New("error"))

	suite.repository.mapper = mapper

	err := suite.repository.Insert(context.TODO(), document)
	assert.Error(suite.T(), err)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_Insert_Ok() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	document2, err := suite.repository.GetById(context.TODO(), document.Id)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), document2)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_Insert_ErrorDb() {
	document := suite.getMerchantDocumentTemplate()

	collectionMock := &mongodbMocks.CollectionInterface{}
	collectionMock.On("InsertOne", mock.Anything, mock.Anything).Return(nil, errors.New("error"))
	dbMock := &mongodbMocks.SourceInterface{}
	dbMock.On("Collection", mock.Anything).Return(collectionMock, nil)
	suite.repository.db = dbMock

	err := suite.repository.Insert(context.TODO(), document)
	assert.Error(suite.T(), err)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_Insert_ErrorMapping() {
	document := suite.getMerchantDocumentTemplate()
	document.CreatedAt = &timestamp.Timestamp{Seconds: -100000000000000}
	err := suite.repository.Insert(context.TODO(), document)
	assert.Error(suite.T(), err)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetById_Ok() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	document2, err := suite.repository.GetById(context.TODO(), document.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), document.Id, document2.Id)
	assert.Equal(suite.T(), document.MerchantId, document2.MerchantId)
	assert.Equal(suite.T(), document.UserId, document2.UserId)
	assert.Equal(suite.T(), document.OriginalName, document2.OriginalName)
	assert.Equal(suite.T(), document.FilePath, document2.FilePath)
	assert.Equal(suite.T(), document.Description, document2.Description)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByIdAndCurrency_MapError() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	mapper := &mocks.Mapper{}
	mapper.On("MapMgoToObject", mock.Anything).Return(nil, errors.New("error"))
	mapper.On("MapObjectToMgo", mock.Anything).Return(nil, errors.New("error"))

	suite.repository.mapper = mapper

	document2, err := suite.repository.GetById(context.TODO(), document.Id)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), document2)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetById_ErrorInvalidId() {
	document, err := suite.repository.GetById(context.TODO(), "id")
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), document)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetById_NotFound() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	document2, err := suite.repository.GetById(context.TODO(), primitive.NewObjectID().Hex())
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), document2)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByMerchantId_Ok() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.GetByMerchantId(context.TODO(), document.MerchantId, 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByMerchantId_OffsetLimit_Ok() {
	document1 := suite.getMerchantDocumentTemplate()
	err := suite.repository.Insert(context.TODO(), document1)
	assert.NoError(suite.T(), err)

	document2 := suite.getMerchantDocumentTemplate()
	document2.MerchantId = document1.MerchantId
	err = suite.repository.Insert(context.TODO(), document2)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.GetByMerchantId(context.TODO(), document1.MerchantId, 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), document1.Id, list[0].Id)
	assert.Equal(suite.T(), document1.MerchantId, list[0].MerchantId)

	list, err = suite.repository.GetByMerchantId(context.TODO(), document1.MerchantId, 1, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), document2.Id, list[0].Id)
	assert.Equal(suite.T(), document2.MerchantId, list[0].MerchantId)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByMerchantId_ErrorInvalidId() {
	list, err := suite.repository.GetByMerchantId(context.TODO(), "id", 0, 1)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), list)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByMerchantId_NotFoundByMerchantId() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.GetByMerchantId(context.TODO(), primitive.NewObjectID().Hex(), 0, 1)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), list)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByMerchantId_ErrorMapping() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	mapper := &mocks.Mapper{}
	mapper.On("MapMgoToObject", mock.Anything).Return(nil, errors.New("error"))
	suite.repository.mapper = mapper

	list, err := suite.repository.GetByMerchantId(context.TODO(), document.MerchantId, 0, 1)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), list)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByMerchantId_ErrorDbFind() {
	document := suite.getMerchantDocumentTemplate()

	collectionMock := &mongodbMocks.CollectionInterface{}
	collectionMock.On("Find", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("error"))

	dbMock := &mongodbMocks.SourceInterface{}
	dbMock.On("Collection", mock.Anything).Return(collectionMock, nil)
	suite.repository.db = dbMock

	list, err := suite.repository.GetByMerchantId(context.TODO(), document.MerchantId, 0, 1)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), list)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_GetByMerchantId_ErrorDbCursor() {
	document := suite.getMerchantDocumentTemplate()

	cursorMock := &mongodbMocks.CursorInterface{}
	cursorMock.On("All", mock.Anything, mock.Anything).Return(errors.New("cursor error"))

	collectionMock := &mongodbMocks.CollectionInterface{}
	collectionMock.On("Find", mock.Anything, mock.Anything, mock.Anything).Return(cursorMock, nil)

	dbMock := &mongodbMocks.SourceInterface{}
	dbMock.On("Collection", mock.Anything).Return(collectionMock, nil)
	suite.repository.db = dbMock

	list, err := suite.repository.GetByMerchantId(context.TODO(), document.MerchantId, 0, 1)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), list)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_CountByIdAndCurrency_Ok() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	count, err := suite.repository.CountByMerchantId(context.TODO(), document.MerchantId)
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 1, count)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_CountByIdAndCurrency_ErrorInvalidId() {
	count, err := suite.repository.CountByMerchantId(context.TODO(), "id")
	assert.Error(suite.T(), err)
	assert.EqualValues(suite.T(), 0, count)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_CountByIdAndCurrency_ErrorDb() {
	document := suite.getMerchantDocumentTemplate()

	collectionMock := &mongodbMocks.CollectionInterface{}
	collectionMock.On("CountDocuments", mock.Anything, mock.Anything).Return(int64(0), errors.New("error"))
	dbMock := &mongodbMocks.SourceInterface{}
	dbMock.On("Collection", mock.Anything).Return(collectionMock, nil)
	suite.repository.db = dbMock

	count, err := suite.repository.CountByMerchantId(context.TODO(), document.MerchantId)
	assert.Error(suite.T(), err)
	assert.EqualValues(suite.T(), 0, count)
}

func (suite *MerchantDocumentTestSuite) TestMerchantDocument_CountByIdAndCurrency_NotFoundByMerchantId() {
	document := suite.getMerchantDocumentTemplate()

	err := suite.repository.Insert(context.TODO(), document)
	assert.NoError(suite.T(), err)

	count, err := suite.repository.CountByMerchantId(context.TODO(), primitive.NewObjectID().Hex())
	assert.NoError(suite.T(), err)
	assert.EqualValues(suite.T(), 0, count)
}

func (suite *MerchantDocumentTestSuite) getMerchantDocumentTemplate() *billingpb.MerchantDocument {
	return &billingpb.MerchantDocument{
		Id:           primitive.NewObjectID().Hex(),
		MerchantId:   primitive.NewObjectID().Hex(),
		UserId:       primitive.NewObjectID().Hex(),
		OriginalName: "original_name",
		FilePath:     "file_path",
		Description:  "description",
	}
}
