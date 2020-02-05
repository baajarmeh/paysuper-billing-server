package repository

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	internalPkg "github.com/paysuper/paysuper-billing-server/internal/pkg"
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
	collectionPaymentChannelCostMerchant = "payment_channel_cost_merchant"

	cachePaymentChannelCostMerchantKeyId = "pccm:id:%s"
	cachePaymentChannelCostMerchantKey   = "pccm:m:%s:n:%s:pc:%s:r:%s:c:%s:mcc:%s"
	cachePaymentChannelCostMerchantAll   = "pccm:all:m:%s"
)

type paymentChannelCostMerchantRepository repository

// NewPaymentChannelCostMerchantRepository create and return an object for working with the price group repository.
// The returned object implements the PaymentChannelCostMerchantRepositoryInterface interface.
func NewPaymentChannelCostMerchantRepository(db mongodb.SourceInterface, cache database.CacheInterface) PaymentChannelCostMerchantRepositoryInterface {
	s := &paymentChannelCostMerchantRepository{db: db, cache: cache}
	return s
}

func (r *paymentChannelCostMerchantRepository) Insert(ctx context.Context, obj *billingpb.PaymentChannelCostMerchant) error {
	obj.MinAmount = tools.FormatAmount(obj.MinAmount)
	obj.MethodFixAmount = tools.FormatAmount(obj.MethodFixAmount)
	obj.MethodPercent = tools.ToPrecise(obj.MethodPercent)
	obj.PsPercent = tools.ToPrecise(obj.PsPercent)
	obj.PsFixedFee = tools.FormatAmount(obj.PsFixedFee)
	obj.CreatedAt = ptypes.TimestampNow()
	obj.UpdatedAt = ptypes.TimestampNow()
	obj.IsActive = true

	_, err := r.db.Collection(collectionPaymentChannelCostMerchant).InsertOne(ctx, obj)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return err
	}

	return r.updateCaches(obj)
}

func (r *paymentChannelCostMerchantRepository) MultipleInsert(ctx context.Context, obj []*billingpb.PaymentChannelCostMerchant) error {
	c := make([]interface{}, len(obj))
	for i, v := range obj {
		if v.Id == "" {
			v.Id = primitive.NewObjectID().Hex()
		}
		v.MinAmount = tools.FormatAmount(v.MinAmount)
		v.MethodFixAmount = tools.FormatAmount(v.MethodFixAmount)
		v.MethodPercent = tools.ToPrecise(v.MethodPercent)
		v.PsPercent = tools.ToPrecise(v.PsPercent)
		v.PsFixedFee = tools.FormatAmount(v.PsFixedFee)
		v.CreatedAt = ptypes.TimestampNow()
		v.UpdatedAt = ptypes.TimestampNow()
		v.IsActive = true
		c[i] = v
	}

	_, err := r.db.Collection(collectionPaymentChannelCostMerchant).InsertMany(ctx, c)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, c),
		)
		return err
	}

	for _, v := range obj {
		if err := r.updateCaches(v); err != nil {
			return err
		}
	}

	return nil
}

func (r *paymentChannelCostMerchantRepository) Update(ctx context.Context, obj *billingpb.PaymentChannelCostMerchant) error {
	obj.MinAmount = tools.FormatAmount(obj.MinAmount)
	obj.MethodFixAmount = tools.FormatAmount(obj.MethodFixAmount)
	obj.MethodPercent = tools.ToPrecise(obj.MethodPercent)
	obj.PsPercent = tools.ToPrecise(obj.PsPercent)
	obj.PsFixedFee = tools.FormatAmount(obj.PsFixedFee)
	obj.UpdatedAt = ptypes.TimestampNow()
	obj.IsActive = true

	oid, err := primitive.ObjectIDFromHex(obj.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldQuery, obj.Id),
		)
	}

	filter := bson.M{"_id": oid}
	_, err = r.db.Collection(collectionPaymentChannelCostMerchant).ReplaceOne(ctx, filter, obj)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationUpdate),
			zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
		)
		return err
	}

	return r.updateCaches(obj)
}

func (r *paymentChannelCostMerchantRepository) GetById(ctx context.Context, id string) (*billingpb.PaymentChannelCostMerchant, error) {
	var c billingpb.PaymentChannelCostMerchant
	key := fmt.Sprintf(cachePaymentChannelCostMerchantKeyId, id)

	if err := r.cache.Get(key, c); err == nil {
		return &c, nil
	}

	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
	}

	filter := bson.M{"_id": oid, "is_active": true}
	err = r.db.Collection(collectionPaymentChannelCostMerchant).FindOne(ctx, filter).Decode(&c)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return nil, err
	}

	_ = r.cache.Set(key, c, 0)

	return &c, nil
}

func (r *paymentChannelCostMerchantRepository) Find(
	ctx context.Context, merchantId, name, payoutCurrency, region, country, mccCode string,
) (c []*internalPkg.PaymentChannelCostMerchantSet, err error) {
	key := fmt.Sprintf(cachePaymentChannelCostMerchantKey, merchantId, name, payoutCurrency, region, country, mccCode)

	if err := r.cache.Get(key, c); err == nil {
		return c, nil
	}

	merchantOid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
	}

	matchQuery := bson.M{
		"merchant_id":     merchantOid,
		"name":            primitive.Regex{Pattern: "^" + name + "$", Options: "i"},
		"payout_currency": payoutCurrency,
		"is_active":       true,
		"mcc_code":        mccCode,
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

	cursor, err := r.db.Collection(collectionPaymentChannelCostMerchant).Aggregate(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	err = cursor.All(ctx, &c)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	if err = r.cache.Set(key, c, 0); err != nil {
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

func (r *paymentChannelCostMerchantRepository) Delete(ctx context.Context, obj *billingpb.PaymentChannelCostMerchant) error {
	obj.UpdatedAt = ptypes.TimestampNow()
	obj.IsActive = false

	oid, err := primitive.ObjectIDFromHex(obj.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldQuery, obj.Id),
		)
	}

	filter := bson.M{"_id": oid}
	_, err = r.db.Collection(collectionPaymentChannelCostMerchant).ReplaceOne(ctx, filter, obj)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationDelete),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return err
	}

	return r.updateCaches(obj)
}

func (r *paymentChannelCostMerchantRepository) GetAllForMerchant(
	ctx context.Context,
	merchantId string,
) ([]*billingpb.PaymentChannelCostMerchant, error) {
	c := []*billingpb.PaymentChannelCostMerchant{}
	key := fmt.Sprintf(cachePaymentChannelCostMerchantAll, merchantId)
	err := r.cache.Get(key, &c)

	if err == nil {
		return c, nil
	}

	merchantOid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
	}

	query := bson.M{"merchant_id": merchantOid, "is_active": true}
	opts := options.Find().
		SetSort(bson.M{"name": 1, "payout_currency": 1, "region": 1, "country": 1, "mcc_code": 1})
	cursor, err := r.db.Collection(collectionPaymentChannelCostMerchant).Find(ctx, query, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	err = cursor.All(ctx, &c)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPaymentChannelCostMerchant),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
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

func (r *paymentChannelCostMerchantRepository) updateCaches(obj *billingpb.PaymentChannelCostMerchant) (err error) {
	groupKeys := []string{
		fmt.Sprintf(cachePaymentChannelCostMerchantKey, obj.MerchantId, obj.Name, obj.PayoutCurrency, obj.Region, obj.Country, obj.MccCode),
		fmt.Sprintf(cachePaymentChannelCostMerchantKey, obj.MerchantId, obj.Name, obj.PayoutCurrency, obj.Region, "", obj.MccCode),
		fmt.Sprintf(cachePaymentChannelCostMerchantAll, obj.MerchantId),
	}

	for _, key := range groupKeys {
		err = r.cache.Delete(key)

		if err != nil {
			zap.L().Error(
				pkg.ErrorCacheQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorCacheFieldCmd, "DELETE"),
				zap.String(pkg.ErrorCacheFieldKey, key),
			)
			return
		}
	}

	keys := []string{
		fmt.Sprintf(cachePaymentChannelCostMerchantKeyId, obj.Id),
	}

	for _, key := range keys {
		err = r.cache.Delete(key)

		if err != nil {
			zap.L().Error(
				pkg.ErrorCacheQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorCacheFieldCmd, "DELETE"),
				zap.String(pkg.ErrorCacheFieldKey, key),
			)
			return
		}
	}

	if obj.IsActive {
		for _, key := range keys {
			err = r.cache.Set(key, obj, 0)

			if err != nil {
				zap.L().Error(
					pkg.ErrorCacheQueryFailed,
					zap.Error(err),
					zap.String(pkg.ErrorCacheFieldCmd, "SET"),
					zap.String(pkg.ErrorCacheFieldKey, key),
					zap.Any(pkg.ErrorCacheFieldData, obj),
				)
				return
			}
		}
	}

	return
}
