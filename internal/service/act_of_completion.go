package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.uber.org/zap"
	"math"
	"time"
)

var (
	invalidActOfCompletionDateFrom = errors.NewBillingServerErrorMsg("aoc000001", "invalid start date the act of completion")
	invalidActOfCompletionDateTo   = errors.NewBillingServerErrorMsg("aoc000002", "invalid end date the act of completion")
	invalidActOfCompletionMerchant = errors.NewBillingServerErrorMsg("aoc000003", "invalid merchant identity the act of completion")
)

func (s *Service) GetActsOfCompletionList(
	ctx context.Context,
	req *billingpb.ActsOfCompletionListRequest,
	rsp *billingpb.ActsOfCompletionListResponse,
) error {

	merchant, err := s.merchantRepository.GetById(ctx, req.MerchantId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = invalidActOfCompletionMerchant

		return nil
	}

	rsp.Items = []*billingpb.ActsOfCompletionListItem{}
	rsp.Status = billingpb.ResponseStatusOk

	if merchant.FirstPaymentAt == nil || merchant.FirstPaymentAt.Seconds <= 0 {
		return nil
	}

	dFrom, err := ptypes.Timestamp(merchant.FirstPaymentAt)
	if err != nil {
		zap.L().Error(
			pkg.ErrorTimeConversion,
			zap.Any(pkg.ErrorTimeConversionMethod, "ptypes.Timestamp"),
			zap.Any(pkg.ErrorTimeConversionValue, merchant.FirstPaymentAt),
			zap.Error(err),
		)
		return err
	}

	monthsCount := monthsCountSince(dFrom)

	rsp.Items = make([]*billingpb.ActsOfCompletionListItem, monthsCount)

	for i := 0; i < monthsCount; i++ {
		b := now.New(dFrom).BeginningOfMonth()
		e := now.New(dFrom).EndOfMonth()

		rsp.Items[monthsCount-(i+1)] = &billingpb.ActsOfCompletionListItem{
			DateTitle: b.Format("2006-01"),
			DateFrom:  b.Format("2006-01-02"),
			DateTo:    e.Format("2006-01-02"),
		}
		dFrom = e.AddDate(0, 0, 1)
	}

	return nil
}

// monthsCountSince calculates the months between now
// and the createdAtTime time.Time value passed
func monthsCountSince(createdAtTime time.Time) int {
	nowTime := time.Now()
	months := 0
	month := createdAtTime.Month()
	for createdAtTime.Before(nowTime) {
		createdAtTime = createdAtTime.Add(time.Hour * 24)
		nextMonth := createdAtTime.Month()
		if nextMonth != month {
			months++
		}
		month = nextMonth
	}

	return months
}

func (s *Service) GetActOfCompletion(
	ctx context.Context,
	req *billingpb.ActOfCompletionRequest,
	rsp *billingpb.ActOfCompletionResponse,
) error {
	dateFrom, err := time.Parse(billingpb.FilterDateFormat, req.DateFrom)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = invalidActOfCompletionDateFrom

		return nil
	}

	dateTo, err := time.Parse(billingpb.FilterDateFormat, req.DateTo)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = invalidActOfCompletionDateTo

		return nil
	}

	dateFrom = now.New(dateFrom).BeginningOfDay()
	dateTo = now.New(dateTo).EndOfDay()

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
	report, _, err := royaltyHandler.buildMerchantRoyaltyReportRoundedAmounts(ctx, merchant, false)
	if err != nil {
		return err
	}

	payoutAmount := report.Summary.ProductsTotal.GrossTotalAmount - report.Summary.ProductsTotal.TotalFees - report.Summary.ProductsTotal.TotalVat
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
