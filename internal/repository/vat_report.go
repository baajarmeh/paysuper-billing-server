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
	"time"
)

const (
	collectionVatReports = "vat_reports"
)

type vatReportRepository repository

// NewVatReportRepository create and return an object for working with the vat reports repository.
// The returned object implements the VatReportRepositoryInterface interface.
func NewVatReportRepository(db mongodb.SourceInterface) VatReportRepositoryInterface {
	s := &vatReportRepository{db: db, mapper: models.NewVatReportMapper()}
	return s
}

func (r *vatReportRepository) Insert(ctx context.Context, vr *billingpb.VatReport) error {
	mgo, err := r.mapper.MapObjectToMgo(vr)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, vr),
		)
		return err
	}

	_, err = r.db.Collection(collectionVatReports).InsertOne(ctx, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationInsert),
			zap.Any(pkg.ErrorDatabaseFieldQuery, vr),
		)
		return err
	}

	return nil
}

func (r *vatReportRepository) Update(ctx context.Context, vr *billingpb.VatReport) error {
	oid, err := primitive.ObjectIDFromHex(vr.Id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.String(pkg.ErrorDatabaseFieldQuery, vr.Id),
		)
		return err
	}

	vr.UpdatedAt = ptypes.TimestampNow()
	mgo, err := r.mapper.MapObjectToMgo(vr)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseMapModelFailed,
			zap.Error(err),
			zap.Any(pkg.ErrorDatabaseFieldQuery, vr),
		)
		return err
	}

	filter := bson.M{"_id": oid}
	_, err = r.db.Collection(collectionVatReports).ReplaceOne(ctx, filter, mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.String(pkg.ErrorDatabaseFieldOperation, pkg.ErrorDatabaseFieldOperationUpdate),
			zap.Any(pkg.ErrorDatabaseFieldQuery, vr),
		)
		return err
	}

	return nil
}

func (r *vatReportRepository) GetById(ctx context.Context, id string) (*billingpb.VatReport, error) {
	oid, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseInvalidObjectId,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.String(pkg.ErrorDatabaseFieldQuery, id),
		)
		return nil, err
	}

	var mgo = models.MgoVatReport{}
	query := bson.M{"_id": oid}
	err = r.db.Collection(collectionVatReports).FindOne(ctx, query).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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
	}

	return obj.(*billingpb.VatReport), nil
}

func (r *vatReportRepository) GetByCountry(ctx context.Context, country string, sort []string, offset, limit int64) ([]*billingpb.VatReport, error) {
	query := bson.M{"country": country}

	if len(sort) == 0 {
		sort = []string{"-date_from"}
	}

	opts := options.Find().
		SetSort(mongodb.ToSortOption(sort)).
		SetLimit(limit).
		SetSkip(offset)
	cursor, err := r.db.Collection(collectionVatReports).Find(ctx, query, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	var mgoVatReports []*models.MgoVatReport
	err = cursor.All(ctx, &mgoVatReports)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	objs := make([]*billingpb.VatReport, len(mgoVatReports))

	for i, obj := range mgoVatReports {
		v, err := r.mapper.MapMgoToObject(obj)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
			)
			return nil, err
		}
		objs[i] = v.(*billingpb.VatReport)
	}

	return objs, nil
}

func (r *vatReportRepository) GetByStatus(ctx context.Context, statuses []string) ([]*billingpb.VatReport, error) {
	query := bson.M{
		"status": bson.M{"$in": statuses},
	}

	opts := options.Find().
		SetSort(bson.M{"country": 1, "status": 1})
	cursor, err := r.db.Collection(collectionVatReports).Find(ctx, query, opts)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	var mgoVatReports []*models.MgoVatReport
	err = cursor.All(ctx, &mgoVatReports)

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return nil, err
	}

	objs := make([]*billingpb.VatReport, len(mgoVatReports))

	for i, obj := range mgoVatReports {
		v, err := r.mapper.MapMgoToObject(obj)
		if err != nil {
			zap.L().Error(
				pkg.ErrorDatabaseMapModelFailed,
				zap.Error(err),
				zap.Any(pkg.ErrorDatabaseFieldQuery, obj),
			)
			return nil, err
		}
		objs[i] = v.(*billingpb.VatReport)
	}

	return objs, nil
}

func (r *vatReportRepository) GetByCountryPeriod(ctx context.Context, country string, dateFrom, dateTo time.Time) (*billingpb.VatReport, error) {
	var mgo = models.MgoVatReport{}

	query := bson.M{
		"country":   country,
		"date_from": dateFrom,
		"date_to":   dateTo,
		"status":    pkg.VatReportStatusThreshold,
	}
	err := r.db.Collection(collectionVatReports).FindOne(ctx, query).Decode(&mgo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, collectionVatReports),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
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
	}

	return obj.(*billingpb.VatReport), nil
}
