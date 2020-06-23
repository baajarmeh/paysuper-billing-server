package repository

import (
	"context"
	"fmt"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
)

const (
	collectionPaymentSystem = "payment_system"

	cachePaymentSystem = "payment_system:id:%s"
)

type paymentSystemRepository repository

// NewPaymentSystemRepository create and return an object for working with the payment system repository.
// The returned object implements the PaymentSystemRepositoryInterface interface.
func NewPaymentSystemRepository(db mongodb.SourceInterface, cache database.CacheInterface) PaymentSystemRepositoryInterface {
	s := &paymentSystemRepository{db: db, cache: cache, mapper: models.NewPaymentSystemMapper()}
	return s
}

func (r *paymentSystemRepository) Insert(ctx context.Context, ps *billingpb.PaymentSystem) error {
	mgo, err := r.mapper.MapObjectToMgo(ps)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, ps),
		)
		return err
	}

	_, err = r.db.Collection(collectionPaymentSystem).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentSystem),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, ps),
		)
		return err
	}

	key := fmt.Sprintf(cachePaymentSystem, ps.Id)
	err = r.cache.Set(key, ps, 0)

	if err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key),
			zap.Any(pkg.ErrorDatabaseFieldQuery, ps),
		)
		return err
	}

	return nil
}

func (r *paymentSystemRepository) MultipleInsert(ctx context.Context, list []*billingpb.PaymentSystem) error {
	objs := make([]interface{}, len(list))

	for i, v := range list {
		mgo, err := r.mapper.MapObjectToMgo(v)

		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, v),
			)
			return err
		}

		objs[i] = mgo
	}

	_, err := r.db.Collection(collectionPaymentSystem).InsertMany(ctx, objs)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentSystem),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, objs),
		)
		return err
	}

	return nil
}

func (r *paymentSystemRepository) Update(ctx context.Context, ps *billingpb.PaymentSystem) error {
	oid, err := primitive.ObjectIDFromHex(ps.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentSystem),
			zap.String(pkg.ErrorDatabaseFieldQuery, ps.Id),
		)
		return err
	}

	mgo, err := r.mapper.MapObjectToMgo(ps)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, ps),
		)
		return err
	}

	filter := bson.M{"_id": oid}
	_, err = r.db.Collection(collectionPaymentSystem).ReplaceOne(ctx, filter, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentSystem),
			zap.Any(pkg.ErrorDatabaseFieldQuery, ps),
		)
		return err
	}

	key := fmt.Sprintf(cachePaymentSystem, ps.Id)
	err = r.cache.Set(key, ps, 0)

	if err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key),
			zap.Any(pkg.ErrorDatabaseFieldQuery, ps),
		)
		return err
	}

	return nil
}

func (r *paymentSystemRepository) GetById(ctx context.Context, id string) (*billingpb.PaymentSystem, error) {
	var c = &billingpb.PaymentSystem{}
	key := fmt.Sprintf(cachePaymentSystem, id)

	if err := r.cache.Get(key, c); err == nil {
		return c, nil
	}

	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentSystem),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	mgo := &models.MgoPaymentSystem{}
	query := bson.M{"_id": oid, "is_active": true}
	err = r.db.Collection(collectionPaymentSystem).FindOne(ctx, query).Decode(mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentSystem),
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

	c = obj.(*billingpb.PaymentSystem)

	if err := r.cache.Set(key, c, 0); err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key),
			zap.Any(pkg.ErrorCacheFieldData, c),
		)
	}

	return c, nil
}
