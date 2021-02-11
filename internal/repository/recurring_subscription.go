package repository

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
)

const (
	collectionRecurringSubscription = "recurring_subscription"
)

type recurringSubscriptionRepository repository

// NewRecurringSubscriptionRepository create and return an object for working with the recurring subscription repository.
// The returned object implements the RecurringSubscriptionRepositoryInterface interface.
func NewRecurringSubscriptionRepository(db mongodb.SourceInterface) RecurringSubscriptionRepositoryInterface {
	s := &recurringSubscriptionRepository{db: db, mapper: models.NewRecurringSubscriptionMapper()}
	return s
}

func (r *recurringSubscriptionRepository) Insert(ctx context.Context, plan *billingpb.RecurringSubscription) error {
	mgo, err := r.mapper.MapObjectToMgo(plan)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, plan),
		)
		return err
	}

	_, err = r.db.Collection(collectionRecurringSubscription).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, plan),
		)
		return err
	}

	return nil
}

func (r *recurringSubscriptionRepository) Update(ctx context.Context, plan *billingpb.RecurringSubscription) error {
	plan.UpdatedAt = ptypes.TimestampNow()
	oid, err := primitive.ObjectIDFromHex(plan.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, plan.Id),
		)
		return err
	}

	mgo, err := r.mapper.MapObjectToMgo(plan)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, plan),
		)
		return err
	}

	filter := bson.M{"_id": oid}
	err = r.db.Collection(collectionRecurringSubscription).FindOneAndReplace(ctx, filter, mgo).Err()

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, plan.Id),
		)
		return err
	}

	return nil
}

func (r *recurringSubscriptionRepository) GetById(ctx context.Context, id string) (*billingpb.RecurringSubscription, error) {
	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	var mgo = models.MgoRecurringSubscription{}
	filter := bson.M{"_id": oid}
	err = r.db.Collection(collectionRecurringSubscription).FindOne(ctx, filter).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	obj, err := r.mapper.MapMgoToObject(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return nil, err
	}

	return obj.(*billingpb.RecurringSubscription), nil
}

func (r *recurringSubscriptionRepository) GetByPlanIdCustomerId(ctx context.Context, planId, customerId string) (*billingpb.RecurringSubscription, error) {
	planOid, err := primitive.ObjectIDFromHex(planId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, planId),
		)
		return nil, err
	}

	customerOid, err := primitive.ObjectIDFromHex(customerId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, customerId),
		)
		return nil, err
	}

	var mgo = models.MgoRecurringSubscription{}
	filter := bson.M{"plan._id": planOid, "customer.id": customerOid}
	err = r.db.Collection(collectionRecurringSubscription).FindOne(ctx, filter).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	obj, err := r.mapper.MapMgoToObject(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return nil, err
	}

	return obj.(*billingpb.RecurringSubscription), nil
}

func (r *recurringSubscriptionRepository) GetActiveByPlanIdCustomerId(ctx context.Context, planId, customerId string) (*billingpb.RecurringSubscription, error) {
	planOid, err := primitive.ObjectIDFromHex(planId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, planId),
		)
		return nil, err
	}

	customerOid, err := primitive.ObjectIDFromHex(customerId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, customerId),
		)
		return nil, err
	}

	var mgo = models.MgoRecurringSubscription{}
	filter := bson.M{"plan._id": planOid, "customer.id": customerOid, "status": billingpb.RecurringSubscriptionStatusActive}
	err = r.db.Collection(collectionRecurringSubscription).FindOne(ctx, filter).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	obj, err := r.mapper.MapMgoToObject(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return nil, err
	}

	return obj.(*billingpb.RecurringSubscription), nil
}

func (r *recurringSubscriptionRepository) FindByCustomerId(ctx context.Context, customerId string) ([]*billingpb.RecurringSubscription, error) {
	customerOid, err := primitive.ObjectIDFromHex(customerId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, customerId),
		)
		return nil, err
	}

	q := bson.M{"customer.id": customerOid}
	cursor, err := r.db.Collection(collectionRecurringSubscription).Find(ctx, q)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, q),
		)
		return nil, err
	}

	var list []*models.MgoRecurringSubscription
	err = cursor.All(ctx, &list)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, q),
		)
		return nil, err
	}

	objs := make([]*billingpb.RecurringSubscription, len(list))

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
		objs[i] = v.(*billingpb.RecurringSubscription)
	}

	return objs, nil
}

func (r *recurringSubscriptionRepository) FindByMerchantIdCustomerId(
	ctx context.Context, merchantId, customerId string,
) ([]*billingpb.RecurringSubscription, error) {
	merchantOid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
		return nil, err
	}

	customerOid, err := primitive.ObjectIDFromHex(customerId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.String(pkg.ErrorDatabaseFieldQuery, customerId),
		)
		return nil, err
	}

	q := bson.M{"plan.merchant_id": merchantOid, "customer.id": customerOid}
	cursor, err := r.db.Collection(collectionRecurringSubscription).Find(ctx, q)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, q),
		)
		return nil, err
	}

	var list []*models.MgoRecurringSubscription
	err = cursor.All(ctx, &list)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, q),
		)
		return nil, err
	}

	objs := make([]*billingpb.RecurringSubscription, len(list))

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
		objs[i] = v.(*billingpb.RecurringSubscription)
	}

	return objs, nil
}
