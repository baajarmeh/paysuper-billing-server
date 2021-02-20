package repository

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"regexp"
	"time"
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

func (r *recurringSubscriptionRepository) FindExpired(ctx context.Context, expireAt time.Time) ([]*billingpb.RecurringSubscription, error) {
	query := bson.M{"expire_at": bson.M{"$lt": expireAt, "$ne": nil}, "status": billingpb.RecurringSubscriptionStatusActive}
	cursor, err := r.db.Collection(collectionRecurringSubscription).Find(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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

func (r *recurringSubscriptionRepository) FindByPlanId(ctx context.Context, planId string) ([]*billingpb.RecurringSubscription, error) {
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

	query := bson.M{"plan._id": planOid}
	cursor, err := r.db.Collection(collectionRecurringSubscription).Find(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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

func (r *recurringSubscriptionRepository) Find(
	ctx context.Context, userId, merchantId, status, quickFilter string, dateFrom, dateTo *time.Time, limit, offset int64,
) (items []*billingpb.RecurringSubscription, err error) {
	query := bson.M{}

	if userId != "" {
		oid, err := primitive.ObjectIDFromHex(userId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldQuery, userId),
			)
			return nil, err
		}

		query["customer.id"] = oid
	}

	if merchantId != "" {
		oid, err := primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return nil, err
		}

		query["plan.merchant_id"] = oid
	}

	if status != "" {
		query["status"] = status
	}

	if dateFrom != nil && dateTo != nil {
		query["created_at"] = bson.M{"$gte": dateFrom, "$lte": dateTo}
	}

	if quickFilter != "" {
		oid, _ := primitive.ObjectIDFromHex(quickFilter)
		pattern := primitive.Regex{Pattern: ".*" + regexp.QuoteMeta(quickFilter) + ".*", Options: "i"}
		query["$or"] = []bson.M{
			{"customer.id": oid},
			{"customer.uuid": bson.M{"$regex": pattern, "$exists": true}},
			{"customer.external_id": bson.M{"$regex": pattern, "$exists": true}},
			{"customer.email": bson.M{"$regex": pattern, "$exists": true}},
			{"customer.phone": bson.M{"$regex": pattern, "$exists": true}},
		}
	}

	if limit <= 0 {
		limit = pkg.DatabaseRequestDefaultLimit
	}

	if offset <= 0 {
		offset = 0
	}

	opts := options.Find().
		SetSort(bson.M{"_id": 1}).
		SetLimit(limit).
		SetSkip(offset)

	cursor, err := r.db.Collection(collectionRecurringSubscription).Find(ctx, query, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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

func (r *recurringSubscriptionRepository) FindCount(
	ctx context.Context, userId, merchantId, status, quickFilter string, dateFrom, dateTo *time.Time,
) (count int64, err error) {
	query := bson.M{}

	if userId != "" {
		oid, err := primitive.ObjectIDFromHex(userId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, CollectionOrderView),
				zap.String(pkg.ErrorDatabaseFieldQuery, userId),
			)
			return 0, err
		}

		query["customer.id"] = oid
	}

	if merchantId != "" {
		oid, err := primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return 0, err
		}

		query["plan.merchant_id"] = oid
	}

	if status != "" {
		query["status"] = status
	}

	if dateFrom != nil && dateTo != nil {
		query["created_at"] = bson.M{"$gte": dateFrom, "$lte": dateTo}
	}

	if quickFilter != "" {
		oid, _ := primitive.ObjectIDFromHex(quickFilter)
		pattern := primitive.Regex{Pattern: ".*" + regexp.QuoteMeta(quickFilter) + ".*", Options: "i"}
		query["$or"] = []bson.M{
			{"customer.id": oid},
			{"customer.uuid": bson.M{"$regex": pattern, "$exists": true}},
			{"customer.external_id": bson.M{"$regex": pattern, "$exists": true}},
			{"customer.email": bson.M{"$regex": pattern, "$exists": true}},
			{"customer.phone": bson.M{"$regex": pattern, "$exists": true}},
		}
	}

	count, err = r.db.Collection(collectionRecurringSubscription).CountDocuments(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringSubscription),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
	}

	return
}
