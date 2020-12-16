package service

import (
	"context"
	"fmt"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-billing-server/internal/repository"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"regexp"
	"time"
)

const (
	orderFailedNotificationQueryFieldMask = "is_notifications_sent.%s"
)

var (
	reportErrorUnknown = errors.NewBillingServerErrorMsg("rp000001", "request processing failed. try request later")
	privateOrderMapper = models.NewOrderViewPrivateMapper()
	publicOrderMapper  = models.NewOrderViewPublicMapper()
	orderMapper        = models.NewOrderMapper()
)

func (s *Service) FindAllOrdersPublic(
	ctx context.Context,
	req *billingpb.ListOrdersRequest,
	rsp *billingpb.ListOrdersPublicResponse,
) error {
	count, orders, err := s.getOrdersList(ctx, req, make([]*billingpb.OrderViewPublic, 1))

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = reportErrorUnknown

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = &billingpb.ListOrdersPublicResponseItem{
		Count: count,
		Items: orders.([]*billingpb.OrderViewPublic),
	}

	return nil
}

func (s *Service) FindAllOrdersPrivate(
	ctx context.Context,
	req *billingpb.ListOrdersRequest,
	rsp *billingpb.ListOrdersPrivateResponse,
) error {
	count, orders, err := s.getOrdersList(ctx, req, make([]*billingpb.OrderViewPrivate, 0))

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = reportErrorUnknown

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = &billingpb.ListOrdersPrivateResponseItem{
		Count: count,
		Items: orders.([]*billingpb.OrderViewPrivate),
	}

	return nil
}

func (s *Service) FindAllOrders(
	ctx context.Context,
	req *billingpb.ListOrdersRequest,
	rsp *billingpb.ListOrdersResponse,
) error {
	count, orders, err := s.getOrdersList(ctx, req, make([]*billingpb.Order, 0))

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = reportErrorUnknown

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = &billingpb.ListOrdersResponseItem{
		Count: count,
		Items: orders.([]*billingpb.Order),
	}

	return nil
}

func (s *Service) GetOrderPublic(
	ctx context.Context,
	req *billingpb.GetOrderRequest,
	rsp *billingpb.GetOrderPublicResponse,
) error {
	order, err := s.orderViewRepository.GetPublicOrderBy(ctx, "", req.OrderId, req.MerchantId)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = orderErrorNotFound

		return nil
	}

	rsp.Item = order
	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetOrderPrivate(
	ctx context.Context,
	req *billingpb.GetOrderRequest,
	rsp *billingpb.GetOrderPrivateResponse,
) error {
	order, err := s.orderViewRepository.GetPrivateOrderBy(ctx, "", req.OrderId, req.MerchantId)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = orderErrorNotFound

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = order

	return nil
}

func (s *Service) getOrdersList(
	ctx context.Context,
	req *billingpb.ListOrdersRequest,
	receiver interface{},
) (int64, interface{}, error) {
	query := make(bson.M)

	if len(req.Merchant) > 0 {
		var merchants []primitive.ObjectID

		for _, v := range req.Merchant {
			oid, _ := primitive.ObjectIDFromHex(v)
			merchants = append(merchants, oid)
		}

		query["project.merchant_id"] = bson.M{"$in": merchants}
	}

	if req.QuickSearch != "" {
		r := primitive.Regex{Pattern: ".*" + regexp.QuoteMeta(req.QuickSearch) + ".*", Options: "i"}

		query["$or"] = []bson.M{
			{"uuid": bson.M{"$regex": r}},
			{"user.external_id": bson.M{"$regex": r, "$exists": true}},
			{"user.email": bson.M{"$regex": r, "$exists": true}},
			{"user.phone": bson.M{"$regex": r, "$exists": true}},
			// {"metadata_values": bson.M{"$regex": r}},
			{"metadata.invoiceId": bson.M{"$regex": r}},
			{"metadata.order_identifier": bson.M{"$regex": r}},
			{"project.name": bson.M{"$elemMatch": bson.M{"value": r}}},
			{"payment_method.name": bson.M{"$regex": r, "$exists": true}},
			{"merchant_info.company_name": bson.M{"$regex": r, "$exists": true}},
		}
	} else {
		if req.Id != "" {
			query["uuid"] = req.Id
		}

		if len(req.Project) > 0 {
			var projects []primitive.ObjectID

			for _, v := range req.Project {
				oid, _ := primitive.ObjectIDFromHex(v)
				projects = append(projects, oid)
			}

			query["project._id"] = bson.M{"$in": projects}
		}

		if len(req.Country) > 0 {
			query["user.address.country"] = bson.M{"$in": req.Country}
		}

		if len(req.PaymentMethod) > 0 {
			var paymentMethod []primitive.ObjectID

			for _, v := range req.PaymentMethod {
				oid, _ := primitive.ObjectIDFromHex(v)
				paymentMethod = append(paymentMethod, oid)
			}

			query["payment_method._id"] = bson.M{"$in": paymentMethod}
		}

		if len(req.Status) > 0 {
			var statuses []string
			var types []string
			for _, status := range req.Status {
				statuses = append(statuses, status)
				switch status {
				case "refunded":
					types = append(types, "refund")
					break
				case "processed":
					statuses = append(statuses, "refunded", "chargeback")
					types = append(types, "order")
					break
				case "chargeback":
					statuses = append(statuses, "chargeback")
					types = append(types, "refund")
					break
				default:
					break
				}
			}
			query["status"] = bson.M{"$in": statuses}
			if len(types) > 0 {
				query["type"] = bson.M{"$in": types}
			}
		}

		if req.Account != "" {
			r := primitive.Regex{Pattern: ".*" + regexp.QuoteMeta(req.Account) + ".*", Options: "i"}
			query["$or"] = []bson.M{
				{"user.external_id": bson.M{"$regex": r}},
				{"user.phone": bson.M{"$regex": r}},
				{"user.email": bson.M{"$regex": r}},
				{"payment_method.card.masked": bson.M{"$regex": r, "$exists": true}},
				{"payment_method.crypto_currency.address": bson.M{"$regex": r, "$exists": true}},
				{"payment_method.wallet.account": bson.M{"$regex": r, "$exists": true}},
				{"uuid": bson.M{"$regex": r}},
			}
		}

		pmDates := make(bson.M)

		if req.PmDateFrom != "" {
			date, err := time.Parse(billingpb.FilterDatetimeFormat, req.PmDateFrom)
			if err != nil {
				zap.L().Error(
					pkg.ErrorTimeConversion,
					zap.Any(pkg.ErrorTimeConversionMethod, "time.Parse"),
					zap.Any(pkg.ErrorTimeConversionValue, req.PmDateFrom),
					zap.Error(err),
				)
				return 0, nil, err
			}
			pmDates["$gte"] = now.New(date).BeginningOfDay()

		}

		if req.PmDateTo != "" {
			date, err := time.Parse(billingpb.FilterDatetimeFormat, req.PmDateTo)
			if err != nil {
				zap.L().Error(
					pkg.ErrorTimeConversion,
					zap.Any(pkg.ErrorTimeConversionMethod, "time.Parse"),
					zap.Any(pkg.ErrorTimeConversionValue, req.PmDateTo),
					zap.Error(err),
				)
				return 0, nil, err
			}
			pmDates["$lte"] = now.New(date).EndOfDay()
		}

		if len(pmDates) > 0 {
			query["pm_order_close_date"] = pmDates
		}

		prjDates := make(bson.M)

		if req.ProjectDateFrom != "" {
			prjDates["$gte"], _ = time.Parse(billingpb.FilterDatetimeFormat, req.ProjectDateFrom)
		}

		if req.ProjectDateTo != "" {
			prjDates["$lte"], _ = time.Parse(billingpb.FilterDatetimeFormat, req.ProjectDateTo)
		}

		if len(prjDates) > 0 {
			query["created_at"] = prjDates
		}

		if req.StatusNotificationFailedFor != "" {
			field := fmt.Sprintf(orderFailedNotificationQueryFieldMask, req.StatusNotificationFailedFor)
			query[field] = false
		}

		if req.MerchantName != "" {
			query["merchant_info.company_name"] = bson.M{
				"$regex":  primitive.Regex{Pattern: ".*" + regexp.QuoteMeta(req.MerchantName) + ".*", Options: "i"},
				"$exists": true,
			}
		}

		if req.RoyaltyReportId != "" {
			query["royalty_report_id"] = req.RoyaltyReportId
		}

		if req.InvoiceId != "" {
			query["$or"] = []bson.M{
				{"metadata.invoiceId": req.InvoiceId},
				{"metadata.order_identifier": req.InvoiceId},
			}
		}
	}

	if req.HideTest == true {
		query["is_production"] = true
	}

	source := repository.CollectionOrder

	count, err := s.db.Collection(source).CountDocuments(ctx, query)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, source),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)

		return 0, nil, err
	}

	aggregateQuery := helper.MakeOrderAggregateQuery(query, repository.CollectionOrderView, req.Sort, req.Offset, req.Limit)

	cursor, err := s.db.Collection(source).Aggregate(ctx, aggregateQuery)

	if err != nil {
		zap.L().Error(
			pkg.ErrorDatabaseQueryFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, source),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return 0, nil, err
	}

	if _, ok := receiver.([]*billingpb.OrderViewPublic); ok {
		var res []*models.MgoOrderViewPublic
		err = cursor.All(ctx, &res)
		temp := make([]*billingpb.OrderViewPublic, len(res))
		for i, mgo := range res {
			obj, _ := publicOrderMapper.MapMgoToObject(mgo)
			temp[i] = obj.(*billingpb.OrderViewPublic)
		}
		receiver = temp
	} else if _, ok := receiver.([]*billingpb.OrderViewPrivate); ok {
		var res []*models.MgoOrderViewPrivate
		err = cursor.All(ctx, &res)
		temp := make([]*billingpb.OrderViewPrivate, len(res))
		for i, mgo := range res {
			obj, _ := privateOrderMapper.MapMgoToObject(mgo)
			temp[i] = obj.(*billingpb.OrderViewPrivate)
		}
		receiver = temp
	} else if _, ok := receiver.([]*billingpb.Order); ok {
		res := make([]*models.MgoOrder, 0)
		err = cursor.All(ctx, &res)
		items := make([]*billingpb.Order, len(res))

		for i, mgo := range res {
			obj, _ := orderMapper.MapMgoToObject(mgo)
			items[i] = obj.(*billingpb.Order)
		}

		receiver = items
	}

	if err != nil {
		zap.L().Error(
			pkg.ErrorQueryCursorExecutionFailed,
			zap.Error(err),
			zap.String(pkg.ErrorDatabaseFieldCollection, source),
			zap.Any(pkg.ErrorDatabaseFieldQuery, query),
		)
		return 0, nil, err
	}

	return count, receiver, nil
}
