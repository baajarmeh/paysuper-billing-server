package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
	"time"
)

var (
	recurringErrorIncorrectCookie    = errors.NewBillingServerErrorMsg("re000001", "customer cookie value is incorrect")
	recurringCustomerNotFound        = errors.NewBillingServerErrorMsg("re000002", "customer not found")
	recurringErrorUnknown            = errors.NewBillingServerErrorMsg("re000003", "unknown error")
	recurringSavedCardNotFount       = errors.NewBillingServerErrorMsg("re000005", "saved card for customer not found")
	recurringErrorProjectNotFound    = errors.NewBillingServerErrorMsg("re000007", "project not found")
	recurringErrorAccessDeny         = errors.NewBillingServerErrorMsg("re000009", "access denied")
	recurringErrorInvalidPeriod      = errors.NewBillingServerErrorMsg("re000010", "invalid recurring period")
	recurringErrorPlanCreate         = errors.NewBillingServerErrorMsg("re000011", "unable to create recurring plan")
	recurringErrorPlanUpdate         = errors.NewBillingServerErrorMsg("re000012", "unable to update recurring plan")
	recurringErrorPlanNotFound       = errors.NewBillingServerErrorMsg("re000013", "recurring plan not found")
	recurringErrorInvalidExpireDate  = errors.NewBillingServerErrorMsg("re000014", "invalid expire dated")
	recurringErrorDeleteSubscription = errors.NewBillingServerErrorMsg("re000015", "unable to delete subscription")
	recurringErrorInvalidDateFilter  = errors.NewBillingServerErrorMsg("re000015", "invalid date")
)

func (s *Service) DeleteSavedCard(
	ctx context.Context,
	req *billingpb.DeleteSavedCardRequest,
	rsp *billingpb.EmptyResponseWithStatus,
) error {
	customer, err := s.decryptBrowserCookie(req.Cookie)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = recurringErrorIncorrectCookie
		return nil
	}

	if customer.CustomerId == "" && customer.VirtualCustomerId == "" {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = recurringCustomerNotFound
		return nil
	}

	if customer.CustomerId != "" {
		_, err = s.getCustomerById(ctx, customer.CustomerId)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = recurringCustomerNotFound
			return nil
		}
	}

	req1 := &recurringpb.DeleteSavedCardRequest{
		Id:    req.Id,
		Token: customer.CustomerId,
	}

	if req1.Token == "" {
		req1.Token = customer.VirtualCustomerId
	}

	rsp1, err := s.rep.DeleteSavedCard(ctx, req1)

	if err != nil {
		zap.L().Error(
			pkg.ErrorGrpcServiceCallFailed,
			zap.Error(err),
			zap.String(errorFieldService, recurringpb.PayOneRepositoryServiceName),
			zap.String(errorFieldMethod, "DeleteSavedCard"),
			zap.Any(errorFieldRequest, req),
		)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorUnknown
		return nil
	}

	if rsp1.Status != billingpb.ResponseStatusOk {
		rsp.Status = rsp1.Status

		if rsp.Status == billingpb.ResponseStatusSystemError {
			zap.L().Error(
				pkg.ErrorGrpcServiceCallFailed,
				zap.String(errorFieldService, recurringpb.PayOneRepositoryServiceName),
				zap.String(errorFieldMethod, "DeleteSavedCard"),
				zap.Any(errorFieldRequest, req),
				zap.Any(pkg.LogFieldResponse, rsp1),
			)

			rsp.Message = recurringErrorUnknown
		} else {
			rsp.Message = recurringSavedCardNotFount
		}

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}
func (s *Service) AddRecurringPlan(ctx context.Context, req *billingpb.RecurringPlan, rsp *billingpb.AddRecurringPlanResponse) error {
	errMsg := s.checkRecurringPeriodPermission(ctx, req.MerchantId, req.ProjectId)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = errMsg
		return nil
	}

	errMsg = s.validateRecurringPlanRequest(req)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = errMsg
		return nil
	}

	req.Id = primitive.NewObjectID().Hex()

	if req.Status == "" {
		req.Status = pkg.RecurringPlanStatusDisabled
	}

	err := s.recurringPlanRepository.Insert(ctx, req)

	if err != nil {
		zap.L().Error(
			"Unable to create recurring plan",
			zap.Error(err),
			zap.Any("plan", req),
		)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorPlanCreate
		return nil
	}

	rsp.Item, _ = s.recurringPlanRepository.GetById(ctx, req.Id)
	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) UpdateRecurringPlan(ctx context.Context, req *billingpb.RecurringPlan, rsp *billingpb.UpdateRecurringPlanResponse) error {
	errMsg := s.checkRecurringPeriodPermission(ctx, req.MerchantId, req.ProjectId)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = errMsg
		return nil
	}

	errMsg = s.validateRecurringPlanRequest(req)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = errMsg
		return nil
	}

	plan, err := s.recurringPlanRepository.GetById(ctx, req.Id)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = recurringErrorPlanNotFound
		return nil
	}

	plan.Name = req.Name
	plan.Description = req.Description
	plan.Tags = req.Tags
	plan.Status = req.Status
	plan.ExternalId = req.ExternalId
	plan.GroupId = req.GroupId
	plan.Charge = req.Charge
	plan.Expiration = req.Expiration
	plan.Trial = req.Trial
	plan.GracePeriod = req.GracePeriod

	err = s.recurringPlanRepository.Update(ctx, req)
	if err != nil {
		zap.L().Error(
			"Unable to update recurring plan",
			zap.Error(err),
			zap.Any("plan", req),
		)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorPlanUpdate
		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) EnableRecurringPlan(ctx context.Context, req *billingpb.EnableRecurringPlanRequest, rsp *billingpb.EnableRecurringPlanResponse) error {
	errMsg := s.checkRecurringPeriodPermission(ctx, req.MerchantId, req.ProjectId)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = errMsg
		return nil
	}

	plan, err := s.recurringPlanRepository.GetById(ctx, req.PlanId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = recurringErrorPlanNotFound
		return nil
	}

	plan.Status = pkg.RecurringPlanStatusActive

	err = s.recurringPlanRepository.Update(ctx, plan)
	if err != nil {
		zap.L().Error(
			"Unable to update recurring plan",
			zap.Error(err),
			zap.Any("plan", plan),
		)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorPlanUpdate
		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) DisableRecurringPlan(ctx context.Context, req *billingpb.DisableRecurringPlanRequest, rsp *billingpb.DisableRecurringPlanResponse) error {
	errMsg := s.checkRecurringPeriodPermission(ctx, req.MerchantId, req.ProjectId)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = errMsg
		return nil
	}

	plan, err := s.recurringPlanRepository.GetById(ctx, req.PlanId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = recurringErrorPlanNotFound
		return nil
	}

	plan.Status = pkg.RecurringPlanStatusDisabled

	err = s.recurringPlanRepository.Update(ctx, plan)
	if err != nil {
		zap.L().Error(
			"Unable to update recurring plan",
			zap.Error(err),
			zap.Any("plan", plan),
		)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorPlanUpdate
		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) DeleteRecurringPlan(ctx context.Context, req *billingpb.DeleteRecurringPlanRequest, rsp *billingpb.DeleteRecurringPlanResponse) error {
	errMsg := s.checkRecurringPeriodPermission(ctx, req.MerchantId, req.ProjectId)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = errMsg
		return nil
	}

	plan, err := s.recurringPlanRepository.GetById(ctx, req.PlanId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = recurringErrorPlanNotFound
		return nil
	}

	plan.DeletedAt = ptypes.TimestampNow()

	err = s.recurringPlanRepository.Update(ctx, plan)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorPlanUpdate
		return nil
	}

	subscriptions, err := s.recurringSubscriptionRepository.FindByPlanId(ctx, plan.Id)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorUnknown
		return nil
	}

	for _, subscription := range subscriptions {
		err = s.recurringBroker.Publish(recurringpb.RecurringSubscriptionDeleteTopic, subscription, amqp.Table{rabbitmq.BrokerMessageRetryCountHeader: int32(0)})

		if err != nil {
			zap.L().Error(
				"Unable to publish delete subscription message",
				zap.Error(err),
				zap.Any("subscription", subscription),
			)
			return err
		}
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetRecurringPlan(ctx context.Context, req *billingpb.GetRecurringPlanRequest, rsp *billingpb.GetRecurringPlanResponse) error {
	errMsg := s.checkRecurringPeriodPermission(ctx, req.MerchantId, req.ProjectId)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = errMsg
		return nil
	}

	plan, err := s.recurringPlanRepository.GetById(ctx, req.PlanId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = recurringErrorPlanNotFound
		return nil
	}

	rsp.Item = plan
	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetRecurringPlans(ctx context.Context, req *billingpb.GetRecurringPlansRequest, rsp *billingpb.GetRecurringPlansResponse) error {
	errMsg := s.checkRecurringPeriodPermission(ctx, req.MerchantId, req.ProjectId)
	if errMsg != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = errMsg
		return nil
	}

	count, err := s.recurringPlanRepository.FindCount(ctx, req.MerchantId, req.ProjectId, req.ExternalId, req.GroupId, req.Query)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorUnknown
		return nil
	}

	if count > 0 {
		rsp.List, err = s.recurringPlanRepository.Find(ctx, req.MerchantId, req.ProjectId, req.ExternalId, req.GroupId, req.Query, req.Offset, req.Limit)
		if err != nil {
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = recurringErrorUnknown
			return nil
		}
	}

	rsp.Count = int32(count)
	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) FindExpiredSubscriptions(ctx context.Context, req *billingpb.FindExpiredSubscriptionsRequest, rsp *billingpb.FindExpiredSubscriptionsResponse) error {
	expireAt, err := time.Parse("2006-01-02", req.ExpireAt)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = recurringErrorInvalidExpireDate

		return nil
	}

	rsp.List, err = s.recurringSubscriptionRepository.FindExpired(ctx, expireAt)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorUnknown
		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) DeleteRecurringSubscription(
	ctx context.Context,
	req *billingpb.DeleteRecurringSubscriptionRequest,
	res *billingpb.EmptyResponseWithStatus,
) error {
	var customerId string

	browserCookie, err := s.findAndParseBrowserCookie(req.Cookie)

	if err != nil {
		res.Status = billingpb.ResponseStatusForbidden
		res.Message = recurringCustomerNotFound
		return nil
	}

	if browserCookie != nil {
		customerId = browserCookie.CustomerId
	}

	subscription, err := s.recurringSubscriptionRepository.GetById(ctx, req.Id)

	if err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = orderErrorRecurringSubscriptionNotFound
		return nil
	}

	if err = s.checkSubscriptionPermission(customerId, req.MerchantId, subscription); err != nil {
		res.Status = billingpb.ResponseStatusForbidden
		res.Message = recurringErrorAccessDeny
		return nil
	}

	orders, err := s.orderRepository.GetBySubscriptionId(ctx, subscription.Id)
	if err != nil || len(orders) < 1 {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = orderErrorNotFound
		return nil
	}

	ps, err := s.paymentSystemRepository.GetById(ctx, orders[0].PaymentMethod.PaymentSystemId)

	if err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = orderErrorPaymentSystemInactive
		return nil
	}

	h, err := s.paymentSystemGateway.GetGateway(ps.Handler)

	if err != nil {
		zap.L().Error(
			"Unable to get payment system gateway",
			zap.Error(err),
			zap.Any("subscription", subscription),
			zap.Any("payment_system", ps),
		)

		res.Status = billingpb.ResponseStatusSystemError
		res.Message = orderErrorPaymentSystemInactive
		return nil
	}

	err = h.DeleteRecurringSubscription(orders[0], subscription)

	if err != nil {
		zap.L().Error(
			"Unable to delete subscription on payment system",
			zap.Error(err),
			zap.Any("subscription", subscription),
			zap.Any("payment_system", ps),
		)

		res.Status = billingpb.ResponseStatusSystemError
		res.Message = recurringErrorDeleteSubscription
		return nil
	}

	subscription.Status = billingpb.RecurringSubscriptionStatusCanceled

	err = s.recurringSubscriptionRepository.Update(ctx, subscription)

	if err != nil {
		zap.L().Error(
			"Unable to delete subscription on recurring service",
			zap.Error(err),
			zap.Any("subscription", subscription),
		)

		res.Status = billingpb.ResponseStatusSystemError
		res.Message = recurringErrorDeleteSubscription
		return nil
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetSubscriptionsOrders(
	ctx context.Context,
	req *billingpb.GetSubscriptionsOrdersRequest,
	rsp *billingpb.GetSubscriptionsOrdersResponse,
) error {
	browserCookie, err := s.findAndParseBrowserCookie(req.Cookie)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = recurringCustomerNotFound
		return nil
	}

	if browserCookie != nil {
		req.UserId = browserCookie.CustomerId
	}

	if req.UserId == "" && req.MerchantId == "" {
		zap.L().Error(
			"unable to identify performer for delete subscription",
			zap.String("customer_id", req.UserId),
			zap.String("merchant_id", req.MerchantId),
		)

		return recurringErrorAccessDeny
	}

	var (
		dateFrom, dateTo *time.Time
	)

	if req.DatetimeFrom != "" {
		date, err := time.Parse(billingpb.FilterDateFormat, req.DatetimeFrom)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = recurringErrorInvalidDateFilter
			return nil
		}

		dateFrom = &date
	}

	if req.DatetimeTo != "" {
		date, err := time.Parse(billingpb.FilterDateFormat, req.DatetimeTo)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = recurringErrorInvalidDateFilter
			return nil
		}

		dateTo = &date
	}

	orders, err := s.orderViewRepository.FindForRecurringSubscriptions(
		ctx,
		req.UserId,
		req.MerchantId,
		req.ProjectId,
		req.SubscriptionId,
		req.Status,
		dateFrom,
		dateTo,
		req.Limit,
		req.Offset,
	)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorUnknown
		return nil
	}

	count, err := s.orderViewRepository.CountForRecurringSubscriptions(
		ctx,
		req.UserId,
		req.MerchantId,
		req.ProjectId,
		req.SubscriptionId,
		req.Status,
		dateFrom,
		dateTo,
	)

	items := make([]*billingpb.SubscriptionOrder, len(orders))
	for i, order := range orders {
		var name []string

		if order.Items != nil {
			for _, item := range order.Items {
				name = append(name, item.Name)
			}
		}

		items[i] = &billingpb.SubscriptionOrder{
			Id:          order.Uuid,
			Amount:      float32(order.OrderCharge.AmountRounded),
			Currency:    order.OrderCharge.Currency,
			Date:        order.TransactionDate,
			CardNumber:  order.GetCardNumber(),
			ProductName: name,
		}
	}

	rsp.List = items
	rsp.Message = nil
	rsp.Status = billingpb.ResponseStatusOk
	rsp.Count = count

	return nil
}

func (s *Service) GetSubscription(
	ctx context.Context,
	req *billingpb.GetSubscriptionRequest,
	rsp *billingpb.GetSubscriptionResponse,
) error {
	var customerId string

	browserCookie, err := s.findAndParseBrowserCookie(req.Cookie)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = recurringCustomerNotFound
		return nil
	}

	if browserCookie != nil {
		customerId = browserCookie.CustomerId
	}

	subscription, err := s.recurringSubscriptionRepository.GetById(ctx, req.Id)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = orderErrorRecurringSubscriptionNotFound
		return nil
	}

	if err = s.checkSubscriptionPermission(customerId, req.MerchantId, subscription); err != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = recurringErrorAccessDeny
		return nil
	}

	rsp.Message = nil
	rsp.Status = billingpb.ResponseStatusOk
	rsp.Subscription = subscription

	return nil
}

func (s *Service) FindSubscriptions(ctx context.Context, req *billingpb.FindSubscriptionsRequest, rsp *billingpb.FindSubscriptionsResponse) error {
	var customerId string

	browserCookie, err := s.findAndParseBrowserCookie(req.Cookie)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = recurringCustomerNotFound
		return nil
	}

	if browserCookie != nil {
		customerId = browserCookie.CustomerId
	}

	if customerId == "" && req.MerchantId == "" {
		zap.L().Error(
			"unable to identify performer for find subscriptions",
			zap.String("cookie", req.Cookie),
			zap.String("merchant_id", req.MerchantId),
		)
		rsp.Status = billingpb.ResponseStatusForbidden
		rsp.Message = recurringErrorAccessDeny
		return nil
	}

	var (
		dateFrom, dateTo *time.Time
	)

	if req.DatetimeFrom != "" {
		date, err := time.Parse(billingpb.FilterDateFormat, req.DatetimeFrom)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = recurringErrorInvalidDateFilter
			return nil
		}

		dateFrom = &date
	}

	if req.DatetimeTo != "" {
		date, err := time.Parse(billingpb.FilterDateFormat, req.DatetimeTo)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = recurringErrorInvalidDateFilter
			return nil
		}

		dateTo = &date
	}

	rsp.Count, err = s.recurringSubscriptionRepository.FindCount(
		ctx, req.UserId, req.MerchantId, req.Status, req.QuickFilter, dateFrom, dateTo,
	)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorUnknown
		return nil
	}

	if rsp.Count > 0 {
		subscriptions, err := s.recurringSubscriptionRepository.Find(
			ctx,
			req.UserId,
			req.MerchantId,
			req.Status,
			req.QuickFilter,
			dateFrom, dateTo,
			req.Offset,
			req.Limit,
		)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = recurringErrorUnknown
			return nil
		}

		rsp.List = subscriptions
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) checkRecurringPeriodPermission(ctx context.Context, merchantId, projectId string) *billingpb.ResponseErrorMessage {
	merchant, err := s.merchantRepository.GetById(ctx, merchantId)
	if err != nil {
		return errorMerchantNotFound
	}

	project, err := s.project.GetById(ctx, projectId)
	if err != nil {
		return recurringErrorProjectNotFound
	}

	if project.MerchantId == "" || project.MerchantId != merchant.Id {
		zap.L().Error(
			"Project don`t owned the merchant",
			zap.Error(err),
			zap.Any("merchant", merchant),
			zap.Any("project", project),
		)

		return recurringErrorAccessDeny
	}

	return nil
}

func (s *Service) validateRecurringPlanRequest(req *billingpb.RecurringPlan) *billingpb.ResponseErrorMessage {
	if !s.checkRecurringPeriod(req.Charge.Period.Type, req.Charge.Period.Value) {
		zap.L().Error(
			"Invalid charge period settings",
			zap.Any("settings", req.Charge.Period),
		)

		return recurringErrorInvalidPeriod
	}

	if req.Expiration != nil && (!s.checkRecurringPeriod(req.Expiration.Type, req.Expiration.Value) ||
		!s.checkRecurringExpirationPeriod(req.Charge.Period, req.Expiration)) {
		zap.L().Error(
			"Invalid expiration period settings",
			zap.Any("expiration", req.Expiration),
			zap.Any("charge", req.Charge.Period),
		)

		return recurringErrorInvalidPeriod
	}

	if req.Trial != nil && !s.checkRecurringPeriod(req.Trial.Type, req.Trial.Value) {
		zap.L().Error(
			"Invalid trial period settings",
			zap.Any("settings", req.Trial),
		)

		return recurringErrorInvalidPeriod
	}

	if req.GracePeriod != nil && !s.checkRecurringPeriod(req.GracePeriod.Type, req.GracePeriod.Value) {
		zap.L().Error(
			"Invalid grace period settings",
			zap.Any("settings", req.GracePeriod),
		)

		return recurringErrorInvalidPeriod
	}

	return nil
}

func (s *Service) checkRecurringPeriod(period string, value int32) bool {
	if value < 1 ||
		period == billingpb.RecurringPeriodMinute && value > 60 ||
		period == billingpb.RecurringPeriodDay && value > 365 ||
		period == billingpb.RecurringPeriodWeek && value > 52 ||
		period == billingpb.RecurringPeriodMonth && value > 12 ||
		period == billingpb.RecurringPeriodYear && value > 1 {
		return false
	}

	return true
}

func (s *Service) checkRecurringExpirationPeriod(chargePeriod, expiration *billingpb.RecurringPlanPeriod) bool {
	var (
		currentTime                        = time.Now().UTC()
		chargeDateTime, expirationDateTime time.Time
	)

	switch chargePeriod.Type {
	case billingpb.RecurringPeriodMinute:
		chargeDateTime = currentTime.Add(time.Minute * time.Duration(chargePeriod.Value))
	case billingpb.RecurringPeriodDay:
		chargeDateTime = currentTime.AddDate(0, 0, int(chargePeriod.Value))
	case billingpb.RecurringPeriodWeek:
		chargeDateTime = currentTime.AddDate(0, 0, int(chargePeriod.Value)*7)
	case billingpb.RecurringPeriodMonth:
		chargeDateTime = currentTime.AddDate(0, int(chargePeriod.Value), 0)
	case billingpb.RecurringPeriodYear:
		chargeDateTime = currentTime.AddDate(int(chargePeriod.Value), 0, 0)
	}

	switch expiration.Type {
	case billingpb.RecurringPeriodMinute:
		expirationDateTime = currentTime.Add(time.Minute * time.Duration(expiration.Value))
	case billingpb.RecurringPeriodDay:
		expirationDateTime = currentTime.AddDate(0, 0, int(expiration.Value))
	case billingpb.RecurringPeriodWeek:
		expirationDateTime = currentTime.AddDate(0, 0, int(expiration.Value)*7)
	case billingpb.RecurringPeriodMonth:
		expirationDateTime = currentTime.AddDate(0, int(expiration.Value), 0)
	case billingpb.RecurringPeriodYear:
		expirationDateTime = currentTime.AddDate(int(expiration.Value), 0, 0)
	}

	if expirationDateTime.Unix() < chargeDateTime.Unix() {
		return false
	}

	return true
}

func (s *Service) findAndParseBrowserCookie(cookie string) (*BrowserCookieCustomer, error) {
	if cookie != "" {
		browserCookie, err := s.decryptBrowserCookie(cookie)
		if err != nil {
			zap.L().Error(
				"can't decrypt cookie",
				zap.Error(err),
				zap.Any(errorFieldRequest, cookie),
			)

			return nil, err
		}

		if len(browserCookie.CustomerId) == 0 {
			zap.L().Error(
				"customer_id is empty",
				zap.Any("browserCookie", browserCookie),
				zap.Any("cookie", cookie),
			)

			return nil, err
		}

		return browserCookie, nil
	}

	return nil, nil
}

func (s *Service) checkSubscriptionPermission(customerId, merchantId string, subscription *billingpb.RecurringSubscription) error {
	if customerId == "" && merchantId == "" {
		zap.L().Error(
			"unable to identify performer for delete subscription",
			zap.String("customerId", customerId),
			zap.String("merchant_id", merchantId),
			zap.Any("subscription", subscription),
		)

		return recurringErrorAccessDeny
	}

	if customerId != "" && customerId != subscription.Customer.Id {
		zap.L().Error(
			"trying to get subscription for another customer",
			zap.String("customer_id", customerId),
			zap.Any("subscription", subscription),
		)

		return recurringErrorAccessDeny
	}

	if merchantId != "" && merchantId != subscription.Plan.MerchantId {
		zap.L().Error(
			"trying to get subscription for another customer",
			zap.String("merchant_id", merchantId),
			zap.Any("subscription", subscription),
		)

		return recurringErrorAccessDeny
	}

	return nil
}
