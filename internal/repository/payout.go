package repository

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	pkg2 "github.com/paysuper/paysuper-billing-server/internal/pkg"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	billErr "github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"math"
	"time"
)

const (
	collectionPayoutDocuments       = "payout_documents"
	collectionPayoutDocumentChanges = "payout_documents_changes"

	cacheKeyPayoutDocument         = "payout_document:id:%s"
	cacheKeyPayoutDocumentMerchant = "payout_document:id:%s:merchant:id:%s"
)

type payoutRepository repository

// NewPayoutRepository create and return an object for working with the payout repository.
// The returned object implements the PayoutRepositoryInterface interface.
func NewPayoutRepository(db mongodb.SourceInterface, cache database.CacheInterface) PayoutRepositoryInterface {
	s := &payoutRepository{db: db, cache: cache, mapper: models.NewPayoutMapper()}
	return s
}

func (r *payoutRepository) Insert(ctx context.Context, pd *billingpb.PayoutDocument, ip, source string) error {
	mgo, err := r.mapper.MapObjectToMgo(pd)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, pd),
		)
		return err
	}

	_, err = r.db.Collection(collectionPayoutDocuments).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldDocument, pd),
		)
		return err
	}

	if err = r.onPayoutDocumentChange(ctx, pd, ip, source); err != nil {
		return err
	}

	return r.updateCaches(pd)
}

func (r *payoutRepository) Update(ctx context.Context, pd *billingpb.PayoutDocument, ip, source string) error {
	oid, err := primitive.ObjectIDFromHex(pd.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldQuery, pd.Id),
		)
		return err
	}

	mgo, err := r.mapper.MapObjectToMgo(pd)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, pd),
		)
		return err
	}

	filter := bson.M{"_id": oid}
	_, err = r.db.Collection(collectionPayoutDocuments).ReplaceOne(ctx, filter, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationUpdate),
			zap.Any(pkg.ErrorDatabaseFieldDocument, pd),
		)

		return err
	}

	if err = r.onPayoutDocumentChange(ctx, pd, ip, source); err != nil {
		return err
	}

	return r.updateCaches(pd)
}

func (r *payoutRepository) GetById(ctx context.Context, id string) (*billingpb.PayoutDocument, error) {
	var c = &billingpb.PayoutDocument{}
	key := fmt.Sprintf(cacheKeyPayoutDocument, id)

	if err := r.cache.Get(key, c); err == nil {
		return c, nil
	}

	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	var mgo = models.MgoPayoutDocument{}
	filter := bson.M{"_id": oid}
	err = r.db.Collection(collectionPayoutDocuments).FindOne(ctx, filter).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldDocumentId, id),
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

	c = obj.(*billingpb.PayoutDocument)

	if err = r.updateCaches(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *payoutRepository) GetByIdMerchantId(ctx context.Context, id, merchantId string) (*billingpb.PayoutDocument, error) {
	var c = &billingpb.PayoutDocument{}
	key := fmt.Sprintf(cacheKeyPayoutDocumentMerchant, id, merchantId)

	if err := r.cache.Get(key, c); err == nil {
		return c, nil
	}

	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	query := bson.M{"_id": oid}

	if merchantId != "" {
		merchantOid, err := primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return nil, err
		}

		query["merchant_id"] = merchantOid
	}

	var mgo = models.MgoPayoutDocument{}
	err = r.db.Collection(collectionPayoutDocuments).FindOne(ctx, query).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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

	c = obj.(*billingpb.PayoutDocument)

	if err = r.updateCaches(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *payoutRepository) GetBalanceAmount(ctx context.Context, merchantId, currency string) (float64, error) {
	oid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
		return float64(0), err
	}

	payoutDocumentStatusActive := []string{
		pkg.PayoutDocumentStatusPending,
		pkg.PayoutDocumentStatusPaid,
	}
	query := []bson.M{
		{
			"$match": bson.M{
				"merchant_id": oid,
				"currency":    currency,
				"status":      bson.M{"$in": payoutDocumentStatusActive},
			},
		},
		{
			"$project": bson.M{
				"currency":   "$currency",
				"total_fees": bson.M{"$round": []interface{}{"$total_fees", 2}},
			},
		},
	}

	result := make([]*pkg2.BalanceQueryResult, 0)
	cursor, err := r.db.Collection(collectionPayoutDocuments).Aggregate(ctx, query)

	if err != nil {
		if err != mongo.ErrNoDocuments {
			zap.L().Error(
				pkg.ErrorDatabaseQueryFailed,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
				zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			)
		}
		return 0, err
	}

	err = cursor.All(ctx, &result)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return 0, err
	}

	if len(result) <= 0 {
		return 0, nil
	}

	balance := make(map[string]float64)

	for _, val := range result {
		balance[val.Currency] += val.TotalFees
	}

	if len(balance) > 1 {
		return 0, billErr.ErrBalanceHasMoreOneCurrency
	}

	amount := math.Round(balance[result[0].Currency]*100) / 100

	return amount, nil
}

func (r *payoutRepository) GetLast(ctx context.Context, merchantId, currency string) (*billingpb.PayoutDocument, error) {
	oid, err := primitive.ObjectIDFromHex(merchantId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
		)
		return nil, err
	}

	payoutDocumentStatusActive := []string{
		pkg.PayoutDocumentStatusPending,
		pkg.PayoutDocumentStatusPaid,
	}
	query := bson.M{
		"merchant_id": oid,
		"currency":    currency,
		"status":      bson.M{"$in": payoutDocumentStatusActive},
	}

	var mgo = models.MgoPayoutDocument{}
	sorts := bson.M{"created_at": -1}
	opts := options.FindOne().SetSort(sorts)
	err = r.db.Collection(collectionPayoutDocuments).FindOne(ctx, query, opts).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
			zap.Any(pkg.ErrorDatabaseFieldSorts, sorts),
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

	return obj.(*billingpb.PayoutDocument), nil
}

func (r *payoutRepository) FindCount(
	ctx context.Context,
	in *billingpb.GetPayoutDocumentsRequest,
) (int64, error) {
	filter, err := r.getFindFilter(in)

	if err != nil {
		return 0, err
	}

	count, err := r.db.Collection(collectionPayoutDocuments).CountDocuments(ctx, filter)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
		)
		return 0, err
	}

	return count, nil
}

func (r *payoutRepository) Find(
	ctx context.Context,
	in *billingpb.GetPayoutDocumentsRequest,
) ([]*billingpb.PayoutDocument, error) {
	filter, err := r.getFindFilter(in)

	if err != nil {
		return nil, err
	}

	opts := options.Find().
		SetSort(bson.M{"created_at": -1}).
		SetLimit(in.Limit).
		SetSkip(in.Offset)
	cursor, err := r.db.Collection(collectionPayoutDocuments).Find(ctx, filter, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
			zap.Any(pkg.ErrorDatabaseFieldLimit, in.Limit),
			zap.Any(pkg.ErrorDatabaseFieldOffset, in.Offset),
		)
		return nil, err
	}

	var mgoPayoutDocuments []*models.MgoPayoutDocument
	err = cursor.All(ctx, &mgoPayoutDocuments)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, filter),
			zap.Any(pkg.ErrorDatabaseFieldLimit, in.Limit),
			zap.Any(pkg.ErrorDatabaseFieldOffset, in.Offset),
		)
		return nil, err
	}

	objs := make([]*billingpb.PayoutDocument, len(mgoPayoutDocuments))

	for i, obj := range mgoPayoutDocuments {
		v, err := r.mapper.MapMgoToObject(obj)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
			)
			return nil, err
		}
		objs[i] = v.(*billingpb.PayoutDocument)
	}

	return objs, nil
}

func (r *payoutRepository) updateCaches(pd *billingpb.PayoutDocument) (err error) {
	key1 := fmt.Sprintf(cacheKeyPayoutDocument, pd.Id)
	key2 := fmt.Sprintf(cacheKeyPayoutDocumentMerchant, pd.Id, pd.MerchantId)

	err = r.cache.Set(key1, pd, 0)
	if err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key1),
			zap.Any(pkg.ErrorCacheFieldData, pd),
		)
		return
	}

	err = r.cache.Set(key2, pd, 0)
	if err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key2),
			zap.Any(pkg.ErrorCacheFieldData, pd),
		)
	}
	return
}

func (r *payoutRepository) onPayoutDocumentChange(
	ctx context.Context,
	document *billingpb.PayoutDocument,
	ip, source string,
) error {
	change := &billingpb.PayoutDocumentChanges{
		Id:               primitive.NewObjectID().Hex(),
		PayoutDocumentId: document.Id,
		Source:           source,
		Ip:               ip,
		CreatedAt:        ptypes.TimestampNow(),
	}

	b, err := json.Marshal(document)

	if err != nil {
		zap.L().Error(
			pkg.ErrorJsonMarshallingFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldDocument, document),
		)
		return err
	}

	hash := md5.New()
	hash.Write(b)
	change.Hash = hex.EncodeToString(hash.Sum(nil))

	mgo, err := models.NewPayoutChangesMapper().MapObjectToMgo(change)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, change),
		)
		return err
	}

	_, err = r.db.Collection(collectionPayoutDocumentChanges).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocumentChanges),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldDocument, mgo),
		)
		return err
	}

	return nil
}

func (r *payoutRepository) FindAll(
	ctx context.Context,
) ([]*billingpb.PayoutDocument, error) {
	query := bson.M{}
	cursor, err := r.db.Collection(collectionPayoutDocuments).Find(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	mgoPayoutDocuments := make([]*models.MgoPayoutDocument, 0)
	err = cursor.All(ctx, &mgoPayoutDocuments)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	objs := make([]*billingpb.PayoutDocument, len(mgoPayoutDocuments))

	for i, obj := range mgoPayoutDocuments {
		v, err := r.mapper.MapMgoToObject(obj)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
			)
			return nil, err
		}
		objs[i] = v.(*billingpb.PayoutDocument)
	}

	return objs, nil
}

func (r *payoutRepository) getFindFilter(in *billingpb.GetPayoutDocumentsRequest) (bson.M, error) {
	filter := make(bson.M)

	if in.MerchantId != "" {
		merchantOid, err := primitive.ObjectIDFromHex(in.MerchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionPayoutDocuments),
				zap.String(pkg.ErrorDatabaseFieldQuery, in.MerchantId),
			)
			return nil, err
		}

		filter = bson.M{"merchant_id": merchantOid}
	}

	if len(in.Status) > 0 {
		filter["status"] = bson.M{"$in": in.Status}
	}

	if in.DateFrom != "" || in.DateTo != "" {
		date := bson.M{}

		if in.DateFrom != "" {
			t, err := time.Parse(billingpb.FilterDatetimeFormat, in.DateFrom)
			if err != nil {
				zap.L().Error(
					pkg.ErrorTimeConversion,
					zap.Any(pkg.ErrorTimeConversionMethod, "time.Parse"),
					zap.Any(pkg.ErrorTimeConversionValue, in.DateFrom),
					zap.Error(err),
				)
				return nil, err
			}
			date["$gte"] = now.New(t).BeginningOfDay()
		}

		if in.DateTo != "" {
			t, err := time.Parse(billingpb.FilterDatetimeFormat, in.DateTo)
			if err != nil {
				zap.L().Error(
					pkg.ErrorTimeConversion,
					zap.Any(pkg.ErrorTimeConversionMethod, "time.Parse"),
					zap.Any(pkg.ErrorTimeConversionValue, in.DateTo),
					zap.Error(err),
				)
				return nil, err
			}
			date["$lte"] = now.New(t).EndOfDay()
		}

		filter["created_at"] = date
	}

	return filter, nil
}
