package service

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	dashboardErrorUnknown = errors.NewBillingServerErrorMsg("db000001", "unknown error. try request later")
)

func (s *Service) GetDashboardMainReport(
	ctx context.Context,
	req *billingpb.GetDashboardMainRequest,
	rsp *billingpb.GetDashboardMainResponse,
) error {
	_, err := s.merchantRepository.GetById(ctx, req.MerchantId)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound

		if err != mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusSystemError
		}

		return nil
	}

	report, err := s.dashboardRepository.GetMainReport(ctx, req.MerchantId, req.Period)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = dashboardErrorUnknown

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = report

	return nil
}

func (s *Service) GetDashboardRevenueDynamicsReport(
	ctx context.Context,
	req *billingpb.GetDashboardMainRequest,
	rsp *billingpb.GetDashboardRevenueDynamicsReportResponse,
) error {
	_, err := s.merchantRepository.GetById(ctx, req.MerchantId)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound

		if err != mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusSystemError
		}

		return nil
	}

	report, err := s.dashboardRepository.GetRevenueDynamicsReport(ctx, req.MerchantId, req.Period)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = dashboardErrorUnknown

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = report

	return nil
}

func (s *Service) GetDashboardBaseReport(
	ctx context.Context,
	req *billingpb.GetDashboardBaseReportRequest,
	rsp *billingpb.GetDashboardBaseReportResponse,
) error {
	_, err := s.merchantRepository.GetById(ctx, req.MerchantId)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound

		if err != mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusSystemError
		}

		return nil
	}

	report, err := s.dashboardRepository.GetBaseReport(ctx, req.MerchantId, req.Period)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = dashboardErrorUnknown

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = report

	return nil
}

func (s *Service) GetDashboardCustomersReport(ctx context.Context, req *billingpb.DashboardCustomerReportRequest, rsp *billingpb.GetDashboardCustomerReportResponse) error {
	report, err := s.dashboardRepository.GetCustomersReport(ctx, req.MerchantId, req.Period)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = merchantErrorBadData

		if err != mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusSystemError
		}

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = report

	return nil
}

func (s *Service) GetDashboardCustomerArpu(ctx context.Context, req *billingpb.DashboardCustomerReportArpuRequest, rsp *billingpb.DashboardCustomerReportArpuResponse) error {
	chart, err := s.dashboardRepository.GetCustomerARPU(ctx, req.MerchantId, req.CustomerId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound

		if err != mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusSystemError
		}

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = chart

	return nil
}
