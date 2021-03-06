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
	"regexp"
	"strings"
)

const (
	collectionProject = "project"

	cacheProjectId = "project:id:%s"
)

type projectRepository repository

// NewProjectRepository create and return an object for working with the price group repository.
// The returned object implements the ProjectRepositoryInterface interface.
func NewProjectRepository(db mongodb.SourceInterface, cache database.CacheInterface) ProjectRepositoryInterface {
	s := &projectRepository{db: db, cache: cache, mapper: models.NewProjectMapper()}
	return s
}

func (r *projectRepository) Insert(ctx context.Context, project *billingpb.Project) error {
	mgo, err := r.mapper.MapObjectToMgo(project)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
		return err
	}

	_, err = r.db.Collection(collectionProject).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
		return err
	}

	key := fmt.Sprintf(cacheProjectId, project.Id)
	err = r.cache.Set(key, project, 0)

	if err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
	}

	return nil
}

func (r *projectRepository) MultipleInsert(ctx context.Context, projects []*billingpb.Project) error {
	p := make([]interface{}, len(projects))
	for i, v := range projects {
		var err error
		p[i], err = r.mapper.MapObjectToMgo(v)
		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, v),
			)
			return err
		}
	}

	_, err := r.db.Collection(collectionProject).InsertMany(ctx, p)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, p),
		)
		return err
	}

	return nil
}

func (r *projectRepository) Update(ctx context.Context, project *billingpb.Project) error {
	oid, err := primitive.ObjectIDFromHex(project.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.String(pkg.ErrorDatabaseFieldQuery, project.Id),
		)
		return err
	}

	filter := bson.M{"_id": oid}
	mgo, err := r.mapper.MapObjectToMgo(project)
	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
		return err
	}

	_, err = r.db.Collection(collectionProject).ReplaceOne(ctx, filter, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationUpdate),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
		return err
	}

	key := fmt.Sprintf(cacheProjectId, project.Id)
	err = r.cache.Set(key, project, 0)

	if err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
		return err
	}

	return nil
}

func (r *projectRepository) GetById(ctx context.Context, id string) (*billingpb.Project, error) {
	project := &billingpb.Project{}
	key := fmt.Sprintf(cacheProjectId, id)
	err := r.cache.Get(key, project)

	if err == nil {
		return project, nil
	}

	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	query := bson.M{"_id": oid}
	var mgo = models.MgoProject{}
	err = r.db.Collection(collectionProject).FindOne(ctx, query).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	obj, err := r.mapper.MapMgoToObject(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
	}

	project = obj.(*billingpb.Project)
	err = r.cache.Set(key, project, 0)

	if err != nil {
		zap.L().Error(
			pkg.ErrorCacheQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorCacheFieldCmd, "SET"),
			zap.String(pkg.ErrorCacheFieldKey, key),
			zap.Any(pkg.ErrorDatabaseFieldQuery, project),
		)
	}

	return project, nil
}

func (r *projectRepository) CountByMerchantId(ctx context.Context, id string) (int64, error) {
	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return int64(0), err
	}

	query := bson.M{"merchant_id": oid}
	count, err := r.db.Collection(collectionProject).CountDocuments(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)

		return int64(0), err
	}

	return count, nil
}

func (r *projectRepository) Find(
	ctx context.Context,
	merchantId,
	quickSearch string,
	statuses []int32,
	offset,
	limit int64,
	sort []string,
) ([]*billingpb.Project, error) {
	var (
		err   error
		query = make(bson.M)
	)

	if merchantId != "" {
		query["merchant_id"], err = primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return nil, err
		}
	}

	if quickSearch != "" {
		pattern := primitive.Regex{Pattern: regexp.QuoteMeta(quickSearch), Options: "i"}
		query["$or"] = []bson.M{
			{"name": bson.M{"$elemMatch": bson.M{"value": pattern}}},
			{"id_string": pattern},
		}
	}

	if len(statuses) > 0 {
		query["status"] = bson.M{"$in": statuses}
	}

	afQuery := []bson.M{
		{"$match": query},
		{
			"$lookup": bson.M{
				"from":         collectionProduct,
				"localField":   "_id",
				"foreignField": "project_id",
				"as":           "products",
			},
		},
		{
			"$project": bson.M{
				"_id":                         "$_id",
				"merchant_id":                 "$merchant_id",
				"name":                        "$name",
				"callback_protocol":           "$callback_protocol",
				"callback_currency":           "$callback_currency",
				"create_order_allowed_urls":   "$create_order_allowed_urls",
				"allow_dynamic_notify_urls":   "$allow_dynamic_notify_urls",
				"allow_dynamic_redirect_urls": "$allow_dynamic_redirect_urls",
				"limits_currency":             "$limits_currency",
				"min_payment_amount":          "$min_payment_amount",
				"max_payment_amount":          "$max_payment_amount",
				"notify_emails":               "$notify_emails",
				"is_products_checkout":        "$is_products_checkout",
				"secret_key":                  "$secret_key",
				"signature_required":          "$signature_required",
				"send_notify_email":           "$send_notify_email",
				"url_check_account":           "$url_check_account",
				"url_process_payment":         "$url_process_payment",
				"url_redirect_fail":           "$url_redirect_fail",
				"url_redirect_success":        "$url_redirect_success",
				"status":                      "$status",
				"created_at":                  "$created_at",
				"updated_at":                  "$updated_at",
				"products_count":              bson.M{"$size": "$products"},
				"cover":                       "$cover",
				"currencies":                  "$currencies",
				"short_description":           "$short_description",
				"full_description":            "$full_description",
				"localizations":               "$localizations",
				"virtual_currency":            "$virtual_currency",
				"vat_payer":                   "$vat_payer",
				"redirect_settings":           "$redirect_settings",
				"webhook_mode":                "$webhook_mode",
			},
		},
		{"$skip": offset},
		{"$limit": limit},
	}

	if len(sort) > 0 {
		pipeSort := make(bson.M)

		for _, field := range sort {
			n := 1

			sField := strings.Split(field, "")

			if sField[0] == "-" {
				n = -1
				field = field[1:]
			}

			pipeSort[field] = n
		}

		if len(pipeSort) > 0 {
			afQuery = append(afQuery, bson.M{"$sort": pipeSort})
		}
	}

	cursor, err := r.db.Collection(collectionProject).Aggregate(ctx, afQuery)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.Any(pkg.ErrorDatabaseFieldQuery, afQuery),
		)
		return nil, err
	}

	var mgoProjects []*models.MgoProject

	err = cursor.All(ctx, &mgoProjects)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	projects := make([]*billingpb.Project, len(mgoProjects))

	for i, project := range mgoProjects {
		obj, err := r.mapper.MapMgoToObject(project)
		if err != nil {
			zap.L().Error(
				pkg.ErrorMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, project),
			)
			return nil, err
		}
		projects[i] = obj.(*billingpb.Project)
	}

	return projects, nil
}

func (r *projectRepository) FindCount(ctx context.Context, merchantId, quickSearch string, statuses []int32) (int64, error) {
	var (
		err   error
		query = make(bson.M)
	)

	if merchantId != "" {
		query["merchant_id"], err = primitive.ObjectIDFromHex(merchantId)

		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseInvalidObjectId,
				zap.Error(err),
				zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
				zap.String(pkg.ErrorDatabaseFieldQuery, merchantId),
			)
			return int64(0), err
		}
	}

	if quickSearch != "" {
		pattern := primitive.Regex{Pattern: regexp.QuoteMeta(quickSearch), Options: "i"}
		query["$or"] = []bson.M{
			{"name": bson.M{"$elemMatch": bson.M{"value": pattern}}},
			{"id_string": pattern},
		}
	}

	if len(statuses) > 0 {
		query["status"] = bson.M{"$in": statuses}
	}

	count, err := r.db.Collection(collectionProject).CountDocuments(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionProject),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return int64(0), err
	}

	return count, nil
}
