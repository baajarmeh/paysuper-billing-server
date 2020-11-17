package repository

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
)

const (
	collectionMerchantDocuments = "merchant_documents"
)

type merchantDocumentRepository repository

// NewMerchantDocumentRepository create and return an object for working with the merchant document repository.
// The returned object implements the MerchantDocumentRepositoryInterface interface.
func NewMerchantDocumentRepository(db mongodb.SourceInterface) MerchantDocumentRepositoryInterface {
	s := &merchantDocumentRepository{db: db, mapper: models.NewMerchantDocumentMapper()}
	return s
}

func (r merchantDocumentRepository) Insert(ctx context.Context, mb *billingpb.MerchantDocument) error {
	mgo, err := r.mapper.MapObjectToMgo(mb)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, mb),
		)
		return err
	}

	_, err = r.db.Collection(collectionMerchantDocuments).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldDocument, mgo),
		)
		return err
	}

	return nil
}

func (r merchantDocumentRepository) GetById(ctx context.Context, id string) (*billingpb.MerchantDocument, error) {
	var (
		mb *billingpb.MerchantDocument
	)

	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.String(pkg.ErrorDatabaseFieldDocumentId, id),
		)
		return nil, err
	}

	query := bson.M{"_id": oid}
	mgo := &models.MgoMerchantDocument{}

	err = r.db.Collection(collectionMerchantDocuments).FindOne(ctx, query).Decode(mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	obj, err := r.mapper.MapMgoToObject(mgo)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
		)
		return nil, err
	}

	mb = obj.(*billingpb.MerchantDocument)

	return mb, nil
}

func (r *merchantDocumentRepository) GetByMerchantId(
	ctx context.Context, merchantId string, offset, limit int64,
) ([]*billingpb.MerchantDocument, error) {
	merchantOid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.String(pkg.ErrorDatabaseFieldDocumentId, merchantId),
		)
		return nil, err
	}

	query := bson.M{"merchant_id": merchantOid}

	opts := options.Find().
		SetSkip(offset).
		SetLimit(limit)
	cursor, err := r.db.Collection(collectionMerchantDocuments).Find(ctx, query, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	var list []*models.MgoMerchantDocument
	err = cursor.All(ctx, &list)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	objs := make([]*billingpb.MerchantDocument, len(list))

	for i, obj := range list {
		v, err := r.mapper.MapMgoToObject(obj)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
			)
			return nil, err
		}
		objs[i] = v.(*billingpb.MerchantDocument)
	}

	return objs, nil
}

func (r merchantDocumentRepository) CountByMerchantId(ctx context.Context, merchantId string) (int64, error) {
	oid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.String(pkg.ErrorDatabaseFieldDocumentId, merchantId),
		)
		return int64(0), err
	}

	query := bson.M{"merchant_id": oid}
	count, err := r.db.Collection(collectionMerchantDocuments).CountDocuments(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMerchantDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return int64(0), err
	}

	return count, nil
}
