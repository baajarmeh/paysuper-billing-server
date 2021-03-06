package repository

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	internalPkg "github.com/paysuper/paysuper-billing-server/internal/pkg"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	tools "github.com/paysuper/paysuper-tools/number"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
)

const (
	collectionMoneyBackCostSystem = "money_back_cost_system"

	cacheMoneyBackCostSystemKey   = "pucs:n:%s:pc:%s:ur:%s:r:%s:c:%s:ps:%d:mcc:%s:oc:%s"
	cacheMoneyBackCostSystemKeyId = "pucs:id:%s"
	cacheMoneyBackCostSystemAll   = "pucs:all"
)

type moneyBackCostSystemRepository repository

// moneyBackCostSystemRepository create and return an object for working with the cost of system for money back.
// The returned object implements the MoneyBackCostSystemRepositoryInterface interface.
func NewMoneyBackCostSystemRepository(db mongodb.SourceInterface, cache database.CacheInterface) MoneyBackCostSystemRepositoryInterface {
	s := &moneyBackCostSystemRepository{db: db, cache: cache, mapper: models.NewMoneyBackCostSystemMapper()}
	return s
}

func (r *moneyBackCostSystemRepository) Insert(ctx context.Context, obj *billingpb.MoneyBackCostSystem) error {
	obj.FixAmount = tools.FormatAmount(obj.FixAmount)
	obj.Percent = tools.ToPrecise(obj.Percent)
	obj.CreatedAt = ptypes.TimestampNow()
	obj.UpdatedAt = ptypes.TimestampNow()
	obj.IsActive = true

	mgo, err := r.mapper.MapObjectToMgo(obj)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return err
	}

	_, err = r.db.Collection(collectionMoneyBackCostSystem).InsertOne(ctx, mgo)

	if err != nil {
		zap.S().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)

		return err
	}

	_ = r.updateCaches(obj)

	return nil
}

func (r *moneyBackCostSystemRepository) MultipleInsert(ctx context.Context, obj []*billingpb.MoneyBackCostSystem) error {
	c := make([]interface{}, len(obj))

	for i, v := range obj {
		v.FixAmount = tools.FormatAmount(v.FixAmount)
		v.Percent = tools.ToPrecise(v.Percent)
		v.CreatedAt = ptypes.TimestampNow()
		v.UpdatedAt = ptypes.TimestampNow()
		v.IsActive = true

		mgo, err := r.mapper.MapObjectToMgo(v)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, v),
			)
		}

		c[i] = mgo
	}

	_, err := r.db.Collection(collectionMoneyBackCostSystem).InsertMany(ctx, c)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, c),
		)
		return err
	}

	for _, v := range obj {
		_ = r.updateCaches(v)
	}

	return nil
}

func (r *moneyBackCostSystemRepository) Update(ctx context.Context, obj *billingpb.MoneyBackCostSystem) error {
	oid, err := primitive.ObjectIDFromHex(obj.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.String(pkg.ErrorDatabaseFieldQuery, obj.Id),
		)
		return err
	}

	obj.FixAmount = tools.FormatAmount(obj.FixAmount)
	obj.Percent = tools.ToPrecise(obj.Percent)
	obj.UpdatedAt = ptypes.TimestampNow()
	obj.IsActive = true

	mgo, err := r.mapper.MapObjectToMgo(obj)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return err
	}

	filter := bson.M{"_id": oid}
	_, err = r.db.Collection(collectionMoneyBackCostSystem).ReplaceOne(ctx, filter, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationUpdate),
			zap.String(pkg.ErrorDatabaseFieldQuery, obj.Id),
		)
		return err
	}

	return r.updateCaches(obj)
}

func (r *moneyBackCostSystemRepository) Find(
	ctx context.Context,
	name string,
	payoutCurrency string,
	undoReason string,
	region string,
	country string,
	mccCode string,
	operatingCompanyId string,
	paymentStage int32,
) (c []*internalPkg.MoneyBackCostSystemSet, err error) {
	key := fmt.Sprintf(
		cacheMoneyBackCostSystemKey,
		name,
		payoutCurrency,
		undoReason,
		region,
		country,
		paymentStage,
		mccCode,
		operatingCompanyId,
	)

	if err := r.cache.Get(key, &c); err == nil {
		return c, nil
	}

	matchQuery := bson.M{
		"name":                 primitive.Regex{Pattern: "^" + name + "$", Options: "i"},
		"payout_currency":      payoutCurrency,
		"undo_reason":          undoReason,
		"payment_stage":        paymentStage,
		"is_active":            true,
		"mcc_code":             mccCode,
		"operating_company_id": operatingCompanyId,
		"$or": []bson.M{
			{
				"country": country,
				"region":  region,
			},
			{
				"$or": []bson.M{
					{"country": ""},
					{"country": bson.M{"$exists": false}},
				},
				"region": region,
			},
		},
	}

	query := []bson.M{
		{
			"$match": matchQuery,
		},
		{
			"$group": bson.M{
				"_id": "$country",
				"set": bson.M{"$push": "$$ROOT"},
			},
		},
		{
			"$sort": bson.M{"_id": -1},
		},
	}

	cursor, err := r.db.Collection(collectionMoneyBackCostSystem).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	var mgo = []*internalPkg.MgoMoneyBackCostSystemSet{}
	err = cursor.All(ctx, &mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	var list = []*billingpb.MoneyBackCostSystem{}

	for _, objs := range mgo {
		list = nil

		for _, obj := range objs.Set {
			v, err := r.mapper.MapMgoToObject(obj)

			if err != nil {
				zap.L().Error(
					pkg.ErrorDatabaseMapModelFailed,
					zap.Error(err),
					zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
				)
				return nil, err
			}

			list = append(list, v.(*billingpb.MoneyBackCostSystem))
		}

		c = append(c, &internalPkg.MoneyBackCostSystemSet{Id: objs.Id, Set: list})
	}

	err = r.cache.Set(key, c, 0)

	if err != nil {
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

func (r *moneyBackCostSystemRepository) GetById(ctx context.Context, id string) (*billingpb.MoneyBackCostSystem, error) {
	c := &billingpb.MoneyBackCostSystem{}
	key := fmt.Sprintf(cacheMoneyBackCostSystemKeyId, id)

	if err := r.cache.Get(key, c); err == nil {
		return c, nil
	}

	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	var mgo = models.MgoMoneyBackCostSystem{}
	filter := bson.M{"_id": oid, "is_active": true}
	err = r.db.Collection(collectionMoneyBackCostSystem).FindOne(ctx, filter).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	obj, err := r.mapper.MapMgoToObject(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, mgo),
		)
		return nil, err
	}

	c = obj.(*billingpb.MoneyBackCostSystem)

	_ = r.cache.Set(key, c, 0)

	return c, nil
}

func (r *moneyBackCostSystemRepository) Delete(ctx context.Context, obj *billingpb.MoneyBackCostSystem) error {
	obj.UpdatedAt = ptypes.TimestampNow()
	obj.IsActive = false

	oid, err := primitive.ObjectIDFromHex(obj.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.String(pkg.ErrorDatabaseFieldQuery, obj.Id),
		)
		return err
	}

	mgo, err := r.mapper.MapObjectToMgo(obj)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return err
	}

	filter := bson.M{"_id": oid}
	_, err = r.db.Collection(collectionMoneyBackCostSystem).ReplaceOne(ctx, filter, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return err
	}

	return r.updateCaches(obj)
}

func (r *moneyBackCostSystemRepository) GetAll(ctx context.Context) (*billingpb.MoneyBackCostSystemList, error) {
	c := &billingpb.MoneyBackCostSystemList{}
	key := cacheMoneyBackCostSystemAll
	err := r.cache.Get(key, c)

	if err != nil {
		filter := bson.M{"is_active": true}
		opts := options.Find().
			SetSort(bson.M{"name": 1, "payout_currency": 1, "undo_reason": 1, "region": 1, "country": 1, "payment_stage": 1})
		cursor, err := r.db.Collection(collectionMoneyBackCostSystem).Find(ctx, filter, opts)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
				zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
			)
			return nil, err
		}

		var list []*models.MgoMoneyBackCostSystem
		err = cursor.All(ctx, &list)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionMoneyBackCostSystem),
				zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
			)
			return nil, err
		}

		objs := make([]*billingpb.MoneyBackCostSystem, len(list))

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
			objs[i] = v.(*billingpb.MoneyBackCostSystem)
		}

		c.Items = objs

		_ = r.cache.Set(key, c, 0)
	}

	return c, nil
}

func (r *moneyBackCostSystemRepository) updateCaches(obj *billingpb.MoneyBackCostSystem) error {
	groupKeys := []string{
		fmt.Sprintf(cacheMoneyBackCostSystemKey, obj.Name, obj.PayoutCurrency, obj.UndoReason, obj.Region, obj.Country, obj.PaymentStage, obj.MccCode, obj.OperatingCompanyId),
		fmt.Sprintf(cacheMoneyBackCostSystemKey, obj.Name, obj.PayoutCurrency, obj.UndoReason, obj.Region, "", obj.PaymentStage, obj.MccCode, obj.OperatingCompanyId),
		cacheMoneyBackCostSystemAll,
	}

	for _, key := range groupKeys {
		if err := r.cache.Delete(key); err != nil {
			return err
		}
	}

	key := fmt.Sprintf(cacheMoneyBackCostSystemKeyId, obj.Id)

	if err := r.cache.Delete(key); err != nil {
		return err
	}

	if obj.IsActive {
		if err := r.cache.Set(key, obj, 0); err != nil {
			zap.L().Error(
				pkg.ErrorCacheQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorCacheFieldCmd, "SET"),
				zap.String(pkg.ErrorCacheFieldKey, key),
				zap.Any(pkg.ErrorCacheFieldData, obj),
			)
			return err
		}
	}

	return nil
}
