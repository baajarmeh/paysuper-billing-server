package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

var (
	recurringErrorIncorrectCookie = errors.NewBillingServerErrorMsg("re000001", "customer cookie value is incorrect")
	recurringCustomerNotFound     = errors.NewBillingServerErrorMsg("re000002", "customer not found")
	recurringErrorUnknown         = errors.NewBillingServerErrorMsg("re000003", "unknown error")
	recurringSavedCardNotFount    = errors.NewBillingServerErrorMsg("re000005", "saved card for customer not found")
	recurringErrorProjectNotFound = errors.NewBillingServerErrorMsg("re000007", "project not found")
	recurringErrorAccessDeny      = errors.NewBillingServerErrorMsg("re000009", "access denied")
	recurringErrorInvalidPeriod   = errors.NewBillingServerErrorMsg("re000010", "invalid recurring period")
	recurringErrorPlanCreate      = errors.NewBillingServerErrorMsg("re000011", "unable to create recurring plan")
	recurringErrorPlanUpdate      = errors.NewBillingServerErrorMsg("re000012", "unable to update recurring plan")
	recurringErrorPlanNotFound    = errors.NewBillingServerErrorMsg("re000013", "recurring plan not found")
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

	rsp.Item = req
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
		zap.L().Error(
			"Unable to update recurring plan",
			zap.Error(err),
			zap.Any("plan", plan),
		)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = recurringErrorPlanUpdate
		return nil
	}

	// TODO: Найти все подписки и погасить их (в том числе в платежной системе)

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

	if req.Expiration != nil && !s.checkRecurringPeriod(req.Expiration.Type, req.Expiration.Value) {
		zap.L().Error(
			"Invalid expiration period settings",
			zap.Any("settings", req.Expiration),
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

func (s *Service) checkRecurringPeriod(typ string, value int32) bool {
	if value < 1 ||
		typ == billingpb.RecurringPeriodMinute && value > 60 ||
		typ == billingpb.RecurringPeriodDay && value > 365 ||
		typ == billingpb.RecurringPeriodWeek && value > 52 ||
		typ == billingpb.RecurringPeriodMonth && value > 12 ||
		typ == billingpb.RecurringPeriodYear && value > 1 {
		return false
	}

	return true
}
