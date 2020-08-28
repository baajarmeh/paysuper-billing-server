package service

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.uber.org/zap"
	"math"
	"time"
)

var (
	invalidActOfCompletionDateFrom = newBillingServerErrorMsg("aoc000001", "invalid start date the act of completion")
	invalidActOfCompletionDateTo   = newBillingServerErrorMsg("aoc000002", "invalid end date the act of completion")
	invalidActOfCompletionMerchant = newBillingServerErrorMsg("aoc000003", "invalid merchant identity the act of completion")
)

func (s *Service) GetActOfCompletion(
	ctx context.Context,
	req *billingpb.ActOfCompletionRequest,
	rsp *billingpb.ActOfCompletionResponse,
) error {
	loc, err := time.LoadLocation(s.cfg.RoyaltyReportTimeZone)

	if err != nil {
		zap.L().Error(royaltyReportErrorTimezoneIncorrect.Error(), zap.Error(err))
		return royaltyReportErrorTimezoneIncorrect
	}

	dateFrom, err := time.Parse("2006-01-02", req.DateFrom)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = invalidActOfCompletionDateFrom

		return nil
	}

	dateTo, err := time.Parse("2006-01-02", req.DateTo)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = invalidActOfCompletionDateTo

		return nil
	}

	dateFrom = time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, loc)
	dateTo = time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, 0, loc)

	merchant, err := s.merchantRepository.GetById(ctx, req.MerchantId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = invalidActOfCompletionMerchant

		return nil
	}

	royaltyHandler := &royaltyHandler{
		Service: s,
		from:    dateFrom,
		to:      dateTo,
	}
	report, _, err := royaltyHandler.buildMerchantRoyaltyReport(ctx, merchant, false)
	if err != nil {
		return err
	}

	grossTotalAmountMoney := helper.NewMoney()
	totalFeesMoney := helper.NewMoney()
	totalVatMoney := helper.NewMoney()

	grossTotalAmount, err := grossTotalAmountMoney.Round(report.Summary.ProductsTotal.GrossTotalAmount)

	if err != nil {
		return err
	}

	totalFees, err := totalFeesMoney.Round(report.Summary.ProductsTotal.TotalFees)

	if err != nil {
		return err
	}

	totalVat, err := totalVatMoney.Round(report.Summary.ProductsTotal.TotalVat)

	if err != nil {

		return err
	}

	payoutAmount := grossTotalAmount - totalFees - totalVat
	totalFeesAmount := payoutAmount + report.Totals.CorrectionAmount
	balanceAmount := payoutAmount + report.Totals.CorrectionAmount - report.Totals.RollingReserveAmount

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = &billingpb.ActOfCompletionDocument{
		MerchantId:        merchant.Id,
		TotalFees:         math.Round(totalFeesAmount*100) / 100,
		Balance:           math.Round(balanceAmount*100) / 100,
		TotalTransactions: report.Totals.TransactionsCount,
	}

	return nil
}
