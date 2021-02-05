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
)

const (
	collectionRecurringPlan = "recurring_plan"
)

type recurringPlanRepository repository

// NewRecurringPlanRepository create and return an object for working with the recurring plan repository.
// The returned object implements the RecurringPlanRepositoryInterface interface.
func NewRecurringPlanRepository(db mongodb.SourceInterface) RecurringPlanRepositoryInterface {
	s := &recurringPlanRepository{db: db, mapper: models.NewRecurringPlanMapper()}
	return s
}

func (r *recurringPlanRepository) Insert(ctx context.Context, plan *billingpb.RecurringPlan) error {
	mgo, err := r.mapper.MapObjectToMgo(plan)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, plan),
		)
		return err
	}

	_, err = r.db.Collection(collectionRecurringPlan).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, plan),
		)
		return err
	}

	return nil
}

func (r *recurringPlanRepository) Update(ctx context.Context, plan *billingpb.RecurringPlan) error {
	plan.UpdatedAt = ptypes.TimestampNow()
	oid, err := primitive.ObjectIDFromHex(plan.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
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
	err = r.db.Collection(collectionRecurringPlan).FindOneAndReplace(ctx, filter, mgo).Err()

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.String(pkg.ErrorDatabaseFieldQuery, plan.Id),
		)
		return err
	}

	return nil
}

func (r *recurringPlanRepository) GetById(ctx context.Context, id string) (*billingpb.RecurringPlan, error) {
	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	var mgo = models.MgoRecurringPlan{}
	filter := bson.M{"_id": oid, "deleted_at": nil}
	err = r.db.Collection(collectionRecurringPlan).FindOne(ctx, filter).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
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

	return obj.(*billingpb.RecurringPlan), nil
}

func (r *recurringPlanRepository) Find(
	ctx context.Context,
	merchantId,
	projectId,
	externalId,
	groupId,
	query string,
	offset,
	limit int32,
) ([]*billingpb.RecurringPlan, error) {
	merchantOid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
		return nil, err
	}

	q := bson.M{"merchant_id": merchantOid, "deleted_at": nil}

	q["project_id"], err = primitive.ObjectIDFromHex(projectId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.String(pkg.ErrorDatabaseFieldQuery, projectId),
		)
		return nil, err
	}

	if externalId != "" {
		q["external_id"] = externalId
	}

	if groupId != "" {
		q["group_id"] = groupId
	}

	if query != "" {
		q["name"] = bson.M{"$elemMatch": bson.M{"value": primitive.Regex{Pattern: regexp.QuoteMeta(query), Options: "i"}}}
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset))
	cursor, err := r.db.Collection(collectionRecurringPlan).Find(ctx, q, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.Any(pkg.ErrorDatabaseFieldQuery, q),
		)
		return nil, err
	}

	var list []*models.MgoRecurringPlan
	err = cursor.All(ctx, &list)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.Any(pkg.ErrorDatabaseFieldQuery, q),
		)
		return nil, err
	}

	objs := make([]*billingpb.RecurringPlan, len(list))

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
		objs[i] = v.(*billingpb.RecurringPlan)
	}

	return objs, nil
}

func (r *recurringPlanRepository) FindCount(
	ctx context.Context,
	merchantId,
	projectId,
	externalId,
	groupId,
	query string,
) (int64, error) {
	merchantOid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
		return int64(0), nil
	}

	q := bson.M{"merchant_id": merchantOid, "deleted_at": nil}

	q["project_id"], err = primitive.ObjectIDFromHex(projectId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.String(pkg.ErrorDatabaseFieldQuery, projectId),
		)
		return int64(0), nil
	}

	if externalId != "" {
		q["external_id"] = externalId
	}

	if groupId != "" {
		q["group_id"] = groupId
	}

	if query != "" {
		q["name"] = bson.M{"$elemMatch": bson.M{"value": primitive.Regex{Pattern: regexp.QuoteMeta(query), Options: "i"}}}
	}

	count, err := r.db.Collection(collectionRecurringPlan).CountDocuments(ctx, q)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionRecurringPlan),
			zap.Any(pkg.ErrorDatabaseFieldQuery, q),
		)
		return 0, nil
	}

	return count, nil
}
