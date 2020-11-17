package service

//1) крон для формирования - 1 раз в неделю (после 18 часов понедельника!)
//2) крон для проверки не пропущена ли дата - каждый день

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	pkg2 "github.com/paysuper/paysuper-billing-server/internal/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/postmarkpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/paysuper/paysuper-proto/go/reporterpb"
	tools "github.com/paysuper/paysuper-tools/number"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"math"
	"mime"
	"net/http"
	"path/filepath"
	"reflect"
	"time"
)

const (
	financeReportsCountForSend = 2
)

var (
	royaltyReportErrorNoTransactions = "no transactions for the period"

	royaltyReportErrorReportNotFound                  = errors.NewBillingServerErrorMsg("rr00001", "royalty report with specified identifier not found")
	royaltyReportErrorReportStatusChangeDenied        = errors.NewBillingServerErrorMsg("rr00002", "change royalty report to new status denied")
	royaltyReportErrorCorrectionReasonRequired        = errors.NewBillingServerErrorMsg("rr00003", "correction reason required")
	royaltyReportEntryErrorUnknown                    = errors.NewBillingServerErrorMsg("rr00004", "unknown error. try request later")
	royaltyReportUpdateBalanceError                   = errors.NewBillingServerErrorMsg("rr00005", "update balance failed")
	royaltyReportErrorTimezoneIncorrect               = errors.NewBillingServerErrorMsg("rr00007", "incorrect time zone")
	royaltyReportErrorAlreadyExistsAndCannotBeUpdated = errors.NewBillingServerErrorMsg("rr00008", "report for this merchant and period already exists and can not be updated")
	royaltyReportErrorCorrectionAmountRequired        = errors.NewBillingServerErrorMsg("rr00009", "correction amount required and must be not zero")
	royaltyReportErrorPayoutDocumentIdInvalid         = errors.NewBillingServerErrorMsg("rr00010", "payout document id is invalid")
	royaltyReportErrorNotOwnedByMerchant              = errors.NewBillingServerErrorMsg("rr00011", "payout document is not owned by merchant")
	royaltyReportErrorMerchantNotFound                = errors.NewBillingServerErrorMsg("rr00012", "royalty report merchant owner not found")

	orderStatusForRoyaltyReports = []string{
		recurringpb.OrderPublicStatusProcessed,
		recurringpb.OrderPublicStatusRefunded,
		recurringpb.OrderPublicStatusChargeback,
	}
)

type royaltyHandler struct {
	*Service
	from time.Time
	to   time.Time
}

func (s *Service) CreateRoyaltyReport(
	ctx context.Context,
	req *billingpb.CreateRoyaltyReportRequest,
	rsp *billingpb.CreateRoyaltyReportRequest,
) error {
	zap.L().Info("start royalty reports processing")

	loc, err := time.LoadLocation(s.cfg.RoyaltyReportTimeZone)

	if err != nil {
		zap.L().Error(royaltyReportErrorTimezoneIncorrect.Error(), zap.Error(err))
		return royaltyReportErrorTimezoneIncorrect
	}

	// prevent to include next day into date - PAY-37849
	to := now.Monday().Add(-1 * time.Nanosecond).In(loc)

	from := to.Add(-time.Duration(s.cfg.RoyaltyReportPeriod) * time.Second).Add(1 * time.Nanosecond).In(loc)
	from = now.New(from).BeginningOfDay()

	var merchants []*pkg2.RoyaltyReportMerchant

	if len(req.Merchants) > 0 {
		for _, v := range req.Merchants {
			oid, err := primitive.ObjectIDFromHex(v)

			if err != nil {
				continue
			}

			merchants = append(merchants, &pkg2.RoyaltyReportMerchant{Id: oid})
		}
	} else {
		merchants, _ = s.orderViewRepository.GetRoyaltyForMerchants(ctx, orderStatusForRoyaltyReports, from, to)
	}

	if len(merchants) <= 0 {
		zap.L().Info(royaltyReportErrorNoTransactions)
		return nil
	}

	handler := &royaltyHandler{
		Service: s,
		from:    from,
		to:      to,
	}

	for _, v := range merchants {
		err := handler.createMerchantRoyaltyReport(ctx, v.Id)

		if err == nil {
			rsp.Merchants = append(rsp.Merchants, v.Id.Hex())
		} else {
			zap.L().Error(
				pkg.ErrorRoyaltyReportGenerationFailed,
				zap.Error(err),
				zap.String(pkg.ErrorRoyaltyReportFieldMerchantId, v.Id.Hex()),
				zap.Any(pkg.ErrorRoyaltyReportFieldFrom, from),
				zap.Any(pkg.ErrorRoyaltyReportFieldTo, to),
			)
		}
	}

	zap.L().Info("royalty reports processing finished successfully")

	return nil
}

func (s *Service) AutoAcceptRoyaltyReports(
	ctx context.Context,
	_ *billingpb.EmptyRequest,
	_ *billingpb.EmptyResponse,
) error {
	reports, err := s.royaltyReportRepository.GetByAcceptedExpireWithStatus(ctx, time.Now(), billingpb.RoyaltyReportStatusPending)

	if err != nil {
		return err
	}

	for _, report := range reports {
		report.Status = billingpb.RoyaltyReportStatusAccepted
		report.AcceptedAt = ptypes.TimestampNow()
		report.UpdatedAt = ptypes.TimestampNow()
		report.IsAutoAccepted = true

		err = s.royaltyReportRepository.Update(ctx, report, "", pkg.RoyaltyReportChangeSourceAuto)
		if err != nil {
			return err
		}

		_, err = s.updateMerchantBalance(ctx, report.MerchantId)
		if err != nil {
			return err
		}

		err = s.royaltyReportChangedEmail(ctx, report)

		if err != nil {
			return err
		}

		err = s.onRoyaltyReportStatusChanged(ctx, report)

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) ListRoyaltyReports(
	ctx context.Context,
	req *billingpb.ListRoyaltyReportsRequest,
	rsp *billingpb.ListRoyaltyReportsResponse,
) error {
	rsp.Status = billingpb.ResponseStatusOk

	count, err := s.royaltyReportRepository.FindCountByMerchantStatusDates(
		ctx, req.MerchantId, req.Status, req.PeriodFrom, req.PeriodTo,
	)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = royaltyReportEntryErrorUnknown

		return nil
	}

	if count <= 0 {
		rsp.Data = &billingpb.RoyaltyReportsPaginate{}
		return nil
	}

	reports, err := s.royaltyReportRepository.FindByMerchantStatusDates(
		ctx, req.MerchantId, req.Status, req.PeriodFrom, req.PeriodTo, req.Offset, req.Limit,
	)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = royaltyReportEntryErrorUnknown

		return nil
	}

	rsp.Data = &billingpb.RoyaltyReportsPaginate{
		Count: count,
		Items: reports,
	}

	return nil
}

func (s *Service) MerchantReviewRoyaltyReport(
	ctx context.Context,
	req *billingpb.MerchantReviewRoyaltyReportRequest,
	rsp *billingpb.ResponseError,
) error {
	report, err := s.royaltyReportRepository.GetById(ctx, req.ReportId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = royaltyReportErrorReportNotFound
			return nil
		}

		return err
	}

	if report.MerchantId != req.MerchantId {
		if err == mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = royaltyReportErrorNotOwnedByMerchant

			return nil
		}
	}

	if report.Status != billingpb.RoyaltyReportStatusPending {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = royaltyReportErrorReportStatusChangeDenied
		return nil
	}

	if req.IsAccepted == true {
		report.Status = billingpb.RoyaltyReportStatusAccepted
		report.AcceptedAt = ptypes.TimestampNow()
	} else {
		report.Status = billingpb.RoyaltyReportStatusDispute
		report.DisputeReason = req.DisputeReason
		report.DisputeStartedAt = ptypes.TimestampNow()
	}

	report.UpdatedAt = ptypes.TimestampNow()

	err = s.royaltyReportRepository.Update(ctx, report, req.Ip, pkg.RoyaltyReportChangeSourceMerchant)

	if err != nil {
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = e
			return nil
		}
		return err
	}

	if req.IsAccepted {
		_, err = s.updateMerchantBalance(ctx, report.MerchantId)
		if err != nil {
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = royaltyReportUpdateBalanceError
			return nil
		}
	}

	err = s.onRoyaltyReportStatusChanged(ctx, report)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = royaltyReportEntryErrorUnknown
		return nil
	}

	err = s.royaltyReportChangedEmail(ctx, report)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = err.(*billingpb.ResponseErrorMessage)
		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetRoyaltyReport(
	ctx context.Context,
	req *billingpb.GetRoyaltyReportRequest,
	rsp *billingpb.GetRoyaltyReportResponse,
) error {
	report, err := s.royaltyReportRepository.GetById(ctx, req.ReportId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = royaltyReportErrorReportNotFound
			return nil
		}

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = royaltyReportEntryErrorUnknown
		return nil
	}

	if req.MerchantId != "" && report.MerchantId != req.MerchantId {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = royaltyReportErrorNotOwnedByMerchant
		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = report

	return nil
}

func (s *Service) ChangeRoyaltyReport(
	ctx context.Context,
	req *billingpb.ChangeRoyaltyReportRequest,
	rsp *billingpb.ResponseError,
) error {
	report, err := s.royaltyReportRepository.GetById(ctx, req.ReportId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = royaltyReportErrorReportNotFound
			return nil
		}

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = royaltyReportEntryErrorUnknown
		return nil
	}

	if report.MerchantId != req.MerchantId {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = royaltyReportErrorNotOwnedByMerchant
		return nil
	}

	if req.Status != "" && report.ChangesAvailable(req.Status) == false {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = royaltyReportErrorReportStatusChangeDenied

		return nil
	}

	hasChanges := false

	if report.Status == billingpb.RoyaltyReportStatusDispute && req.Correction != nil {

		if req.Correction.Reason == "" {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = royaltyReportErrorCorrectionReasonRequired

			return nil
		}

		if req.Correction.Amount == 0 {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = royaltyReportErrorCorrectionAmountRequired

			return nil
		}

		from, err := ptypes.Timestamp(report.PeriodFrom)
		if err != nil {
			zap.L().Error("time conversion error", zap.Error(err))
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = royaltyReportEntryErrorUnknown
			return nil
		}
		to, err := ptypes.Timestamp(report.PeriodTo)
		if err != nil {
			zap.L().Error("time conversion error", zap.Error(err))
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = royaltyReportEntryErrorUnknown
			return nil
		}

		reqAe := &billingpb.CreateAccountingEntryRequest{
			MerchantId: report.MerchantId,
			Amount:     req.Correction.Amount,
			Currency:   report.Currency,
			Reason:     req.Correction.Reason,
			Date:       to.Add(-1 * time.Second).Unix(),
			Type:       pkg.AccountingEntryTypeMerchantRoyaltyCorrection,
		}
		resAe := &billingpb.CreateAccountingEntryResponse{}
		err = s.CreateAccountingEntry(ctx, reqAe, resAe)
		if err != nil {
			zap.L().Error("create correction accounting entry failed", zap.Error(err))
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = royaltyReportEntryErrorUnknown
			return nil
		}
		if resAe.Status != billingpb.ResponseStatusOk {
			zap.L().Error("create correction accounting entry failed")
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = royaltyReportEntryErrorUnknown
			return nil
		}

		if report.Totals == nil {
			report.Totals = &billingpb.RoyaltyReportTotals{}
		}
		if report.Summary == nil {
			report.Summary = &billingpb.RoyaltyReportSummary{}
		}

		handler := &royaltyHandler{
			Service: s,
			from:    from,
			to:      to,
		}
		report.Summary.Corrections, report.Totals.CorrectionAmount, err = handler.getRoyaltyReportCorrections(ctx, report.MerchantId, report.Currency)
		if err != nil {
			zap.L().Error("get royalty report corrections error", zap.Error(err))
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = royaltyReportEntryErrorUnknown
			return nil
		}

		hasChanges = true
	}

	if req.Status != "" && req.Status != report.Status {
		if report.Status == billingpb.RoyaltyReportStatusDispute {
			report.DisputeClosedAt = ptypes.TimestampNow()
		}

		if req.Status == billingpb.RoyaltyReportStatusAccepted {
			report.AcceptedAt = ptypes.TimestampNow()
		}

		report.Status = req.Status

		hasChanges = true
	}

	if hasChanges != true {
		rsp.Status = billingpb.ResponseStatusNotModified
		return nil
	}

	report.UpdatedAt = ptypes.TimestampNow()

	err = s.royaltyReportRepository.Update(ctx, report, req.Ip, pkg.RoyaltyReportChangeSourceAdmin)
	if err != nil {
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = e
			return nil
		}
		return err
	}

	s.sendRoyaltyReportNotification(ctx, report)

	_, err = s.updateMerchantBalance(ctx, report.MerchantId)
	if err != nil {
		return err
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) ListRoyaltyReportOrders(
	ctx context.Context,
	req *billingpb.ListRoyaltyReportOrdersRequest,
	res *billingpb.TransactionsResponse,
) error {
	res.Status = billingpb.ResponseStatusOk

	report, err := s.royaltyReportRepository.GetById(ctx, req.ReportId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = royaltyReportErrorReportNotFound

			return nil
		}

		res.Status = billingpb.ResponseStatusSystemError
		res.Message = royaltyReportEntryErrorUnknown
		return nil
	}

	if err == mongo.ErrNoDocuments {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = royaltyReportErrorReportNotFound
		return nil
	}

	from, _ := ptypes.Timestamp(report.PeriodFrom)
	to, _ := ptypes.Timestamp(report.PeriodTo)

	merchantOid, _ := primitive.ObjectIDFromHex(report.MerchantId)
	match := bson.M{
		"merchant_id":         merchantOid,
		"pm_order_close_date": bson.M{"$gte": from, "$lte": to},
		"status":              bson.M{"$in": orderStatusForRoyaltyReports},
		"is_production":       true,
		"royalty_report_id":   report.Id,
	}

	ts, err := s.orderViewRepository.GetTransactionsPublic(ctx, match, req.Limit, req.Offset)

	if err != nil {
		return err
	}

	res.Data = &billingpb.TransactionsPaginate{
		Count: int32(len(ts)),
		Items: ts,
	}

	return nil
}

func (h *royaltyHandler) getRoyaltyReportCorrections(ctx context.Context, merchantId, currency string) (
	entries []*billingpb.RoyaltyReportCorrectionItem,
	total float64,
	err error) {

	accountingEntries, err := h.accountingRepository.GetCorrectionsForRoyaltyReport(ctx, merchantId, currency, h.from, h.to)
	if err != nil {
		return
	}

	for _, e := range accountingEntries {
		entries = append(entries, &billingpb.RoyaltyReportCorrectionItem{
			AccountingEntryId: e.Id,
			Amount:            e.AmountRounded,
			Reason:            e.Reason,
			EntryDate:         e.CreatedAt,
		})
		total += e.AmountRounded
	}

	return
}

func (h *royaltyHandler) getRoyaltyReportRollingReserves(
	ctx context.Context,
	merchantId, currency string,
) (
	entries []*billingpb.RoyaltyReportCorrectionItem,
	total float64,
	err error) {

	accountingEntries, err := h.accountingRepository.GetRollingReservesForRoyaltyReport(
		ctx, merchantId, currency, rollingReserveAccountingEntriesList, h.from, h.to,
	)

	if err != nil {
		return
	}

	for _, e := range accountingEntries {
		entries = append(entries, &billingpb.RoyaltyReportCorrectionItem{
			AccountingEntryId: e.Id,
			Amount:            e.AmountRounded,
			Reason:            e.Reason,
			EntryDate:         e.CreatedAt,
		})
		total += e.AmountRounded
	}

	return
}

func (h *royaltyHandler) createMerchantRoyaltyReport(ctx context.Context, merchantId primitive.ObjectID) error {
	zap.L().Info("start generating royalty reports for merchant", zap.String("merchant_id", merchantId.Hex()))

	merchant, err := h.merchantRepository.GetById(ctx, merchantId.Hex())
	if err != nil {
		return merchantErrorNotFound
	}

	existingReport := h.royaltyReportRepository.GetReportExists(ctx, merchant.Id, merchant.GetPayoutCurrency(), h.from, h.to)
	if existingReport != nil && existingReport.Status != billingpb.RoyaltyReportStatusPending {
		return royaltyReportErrorAlreadyExistsAndCannotBeUpdated
	}

	newReport, ordersIds, err := h.buildMerchantRoyaltyReportRoundedAmounts(ctx, merchant, true)
	if err != nil {
		return err
	}

	if existingReport != nil {
		newReport.Id = existingReport.Id
		newReport.CreatedAt = existingReport.CreatedAt
		newReport.UpdatedAt = existingReport.UpdatedAt
		newReport.PayoutDate = existingReport.PayoutDate
		newReport.Status = existingReport.Status
		newReport.AcceptExpireAt = existingReport.AcceptExpireAt
		newReport.AcceptedAt = existingReport.AcceptedAt
		newReport.DisputeReason = existingReport.DisputeReason
		newReport.DisputeStartedAt = existingReport.DisputeStartedAt
		newReport.DisputeClosedAt = existingReport.DisputeClosedAt
		newReport.IsAutoAccepted = existingReport.IsAutoAccepted
		newReport.PayoutDocumentId = existingReport.PayoutDocumentId

		if reflect.DeepEqual(existingReport, newReport) {
			zap.L().Info("royalty report exists and unchanged",
				zap.String("merchant_id", newReport.MerchantId),
				zap.String("operating_company_id", newReport.OperatingCompanyId),
			)
			return nil
		}

		newReport.UpdatedAt = ptypes.TimestampNow()

		zap.L().Info("updating exists royalty report",
			zap.String("merchant_id", newReport.MerchantId),
			zap.String("operating_company_id", newReport.OperatingCompanyId),
		)

		err = h.royaltyReportRepository.Update(ctx, newReport, "", pkg.RoyaltyReportChangeSourceAuto)
		if err != nil {
			return err
		}

	} else {
		zap.L().Info("saving new royalty report",
			zap.String("merchant_id", newReport.MerchantId),
			zap.String("operating_company_id", newReport.OperatingCompanyId),
		)

		err = h.royaltyReportRepository.Insert(ctx, newReport, "", pkg.RoyaltyReportChangeSourceAuto)
		if err != nil {
			return err
		}

		err = h.markOrdersIncludedToRoyaltyReport(ctx, newReport.Id, ordersIds)
		if err != nil {
			return err
		}
	}

	err = h.Service.renderRoyaltyReport(ctx, newReport, merchant)
	if err != nil {
		return err
	}

	zap.L().Info("generating royalty reports for merchant finished", zap.String("merchant_id", merchantId.Hex()))

	return nil
}

func (h *royaltyHandler) buildMerchantRoyaltyReport(
	ctx context.Context, merchant *billingpb.Merchant, hasExistsReportId bool,
) (*billingpb.RoyaltyReport, []primitive.ObjectID, error) {
	merchantPayoutCurrency := merchant.GetPayoutCurrency()
	summaryItems, summaryTotal, ordersIds, err := h.orderViewRepository.GetRoyaltySummary(
		ctx,
		merchant.Id,
		merchantPayoutCurrency,
		h.from,
		h.to,
		hasExistsReportId,
	)

	if err != nil {
		return nil, nil, err
	}

	corrections, correctionsTotal, err := h.getRoyaltyReportCorrections(ctx, merchant.Id, merchantPayoutCurrency)
	if err != nil {
		return nil, nil, err
	}

	reserves, reservesTotal, err := h.getRoyaltyReportRollingReserves(ctx, merchant.Id, merchantPayoutCurrency)
	if err != nil {
		return nil, nil, err
	}

	report := &billingpb.RoyaltyReport{
		Id:                 primitive.NewObjectID().Hex(),
		MerchantId:         merchant.Id,
		OperatingCompanyId: merchant.OperatingCompanyId,
		Currency:           merchant.GetPayoutCurrency(),
		Status:             billingpb.RoyaltyReportStatusPending,
		CreatedAt:          ptypes.TimestampNow(),
		UpdatedAt:          ptypes.TimestampNow(),
		Totals: &billingpb.RoyaltyReportTotals{
			TransactionsCount:    summaryTotal.TotalTransactions,
			FeeAmount:            summaryTotal.TotalFees,
			VatAmount:            summaryTotal.TotalVat,
			PayoutAmount:         summaryTotal.PayoutAmount,
			CorrectionAmount:     tools.ToPrecise(correctionsTotal),
			RollingReserveAmount: tools.ToPrecise(reservesTotal),
		},
		Summary: &billingpb.RoyaltyReportSummary{
			ProductsItems:   summaryItems,
			ProductsTotal:   summaryTotal,
			Corrections:     corrections,
			RollingReserves: reserves,
		},
	}

	totalEndUserFeesMoney := helper.NewMoney()
	returnsAmountMoney := helper.NewMoney()
	endUserFeesMoney := helper.NewMoney()
	vatOnEndUserSalesMoney := helper.NewMoney()
	licenseRevenueShareMoney := helper.NewMoney()

	totalGrossSalesAmount := float64(0)
	totalGrossReturnsAmount := float64(0)
	totalGrossTotalAmount := float64(0)
	totalVat := float64(0)
	totalFees := float64(0)
	totalPayoutAmount := float64(0)

	for _, item := range summaryItems {
		grossSalesAmount, err := totalEndUserFeesMoney.Round(item.GrossSalesAmount)

		if err != nil {
			return nil, nil, err
		}

		grossReturnsAmount, err := returnsAmountMoney.Round(item.GrossReturnsAmount)

		if err != nil {
			return nil, nil, err
		}

		grossTotalAmount, err := endUserFeesMoney.Round(item.GrossTotalAmount)

		if err != nil {
			return nil, nil, err
		}

		vat, err := vatOnEndUserSalesMoney.Round(item.TotalVat)

		if err != nil {
			return nil, nil, err
		}

		fees, err := licenseRevenueShareMoney.Round(item.TotalFees)

		if err != nil {
			return nil, nil, err
		}

		payoutAmount := grossTotalAmount - vat - fees

		totalGrossSalesAmount += grossSalesAmount
		totalGrossReturnsAmount += grossReturnsAmount
		totalGrossTotalAmount += grossTotalAmount
		totalVat += vat
		totalFees += fees
		totalPayoutAmount += payoutAmount
	}

	report.Totals.FeeAmount = math.Round(totalFees*100) / 100
	report.Totals.VatAmount = math.Round(totalVat*100) / 100
	report.Totals.PayoutAmount = math.Round(totalPayoutAmount*100) / 100
	report.Summary.ProductsTotal.GrossSalesAmount = math.Round(totalGrossSalesAmount*100) / 100
	report.Summary.ProductsTotal.GrossReturnsAmount = math.Round(totalGrossReturnsAmount*100) / 100
	report.Summary.ProductsTotal.GrossTotalAmount = math.Round(totalGrossTotalAmount*100) / 100
	report.Summary.ProductsTotal.TotalVat = math.Round(totalVat*100) / 100
	report.Summary.ProductsTotal.TotalFees = math.Round(totalFees*100) / 100
	report.Summary.ProductsTotal.PayoutAmount = math.Round(totalPayoutAmount*100) / 100

	report.PeriodFrom, err = ptypes.TimestampProto(h.from)
	if err != nil {
		return nil, nil, err
	}
	report.StringPeriodFrom = h.from.Format("2006-01-02")

	report.PeriodTo, err = ptypes.TimestampProto(h.to)
	if err != nil {
		return nil, nil, err
	}

	// temporary f*cking magic, will be deleted after switch royalty reports generation to UTC time
	to, err := ptypes.Timestamp(report.PeriodTo)
	if err != nil {
		return nil, nil, err
	}
	to = now.New(to).EndOfDay()
	report.StringPeriodTo = to.Format("2006-01-02")

	report.AcceptExpireAt, err = ptypes.TimestampProto(time.Now().Add(time.Duration(h.cfg.RoyaltyReportAcceptTimeout) * time.Second))
	if err != nil {
		return nil, nil, err
	}

	return report, ordersIds, nil
}

func (s *Service) renderRoyaltyReport(
	ctx context.Context,
	report *billingpb.RoyaltyReport,
	merchant *billingpb.Merchant,
) error {
	params, err := json.Marshal(map[string]interface{}{reporterpb.ParamsFieldId: report.Id})
	if err != nil {
		zap.L().Error(
			"Unable to marshal the params of royalty report for the reporting service.",
			zap.Error(err),
		)
		return err
	}

	fileReq := &reporterpb.ReportFile{
		UserId:           merchant.User.Id,
		MerchantId:       merchant.Id,
		ReportType:       reporterpb.ReportTypeRoyalty,
		FileType:         reporterpb.OutputExtensionPdf,
		Params:           params,
		SendNotification: true,
	}

	err = s.reporterServiceCreateFile(ctx, fileReq)
	return err
}

func (s *Service) RoyaltyReportPdfUploaded(
	ctx context.Context,
	req *billingpb.RoyaltyReportPdfUploadedRequest,
	res *billingpb.RoyaltyReportPdfUploadedResponse,
) error {
	report, err := s.royaltyReportRepository.GetById(ctx, req.RoyaltyReportId)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = royaltyReportErrorReportNotFound
			return nil
		}
		return err
	}

	merchant, err := s.merchantRepository.GetById(ctx, report.MerchantId)

	if err != nil {
		zap.L().Error("Merchant not found", zap.Error(err), zap.String("merchant_id", report.MerchantId))
		return merchantErrorNotFound
	}

	if merchant.HasAuthorizedEmail() == false {
		zap.L().Warn("Merchant has no authorized email", zap.String("merchant_id", merchant.Id))
		res.Status = billingpb.ResponseStatusOk
		return nil
	}

	content := base64.StdEncoding.EncodeToString(req.Content)
	contentType := mime.TypeByExtension(filepath.Ext(req.Filename))

	operatingCompany, err := s.operatingCompanyRepository.GetById(ctx, report.OperatingCompanyId)
	if err != nil {
		zap.L().Error("Operating company not found", zap.Error(err), zap.String("operating_company_id", report.OperatingCompanyId))
		return err
	}

	payload := &postmarkpb.Payload{
		TemplateAlias: s.cfg.EmailTemplates.NewRoyaltyReport,
		TemplateModel: map[string]string{
			"merchant_id":            merchant.Id,
			"royalty_report_id":      report.Id,
			"period_from":            report.StringPeriodFrom,
			"period_to":              report.StringPeriodTo,
			"license_agreement":      merchant.AgreementNumber,
			"status":                 report.Status,
			"merchant_greeting":      merchant.GetOwnerName(),
			"royalty_reports_url":    s.cfg.GetRoyaltyReportsUrl(),
			"royalty_report_url":     s.cfg.GetRoyaltyReportUrl(report.Id),
			"operating_company_name": operatingCompany.Name,
			"current_year":           time.Now().UTC().Format("2006"),
		},
		To: merchant.GetOwnerEmail(),
		Attachments: []*postmarkpb.PayloadAttachment{
			{
				Name:        req.Filename,
				Content:     content,
				ContentType: contentType,
			},
		},
	}

	err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, payload, amqp.Table{})

	if err != nil {
		zap.L().Error(
			"Publication message about merchant new payout document to queue failed",
			zap.Error(err),
			zap.Any("report", report),
		)
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) sendRoyaltyReportNotification(ctx context.Context, report *billingpb.RoyaltyReport) {
	merchant, err := s.merchantRepository.GetById(ctx, report.MerchantId)

	if err != nil {
		zap.L().Error("Merchant not found", zap.Error(err), zap.String("merchant_id", report.MerchantId))
		return
	}

	if merchant.HasAuthorizedEmail() == true {
		payload := &postmarkpb.Payload{
			TemplateAlias: s.cfg.EmailTemplates.UpdateRoyaltyReport,
			TemplateModel: map[string]string{
				"merchant_id":         merchant.Id,
				"royalty_report_id":   report.Id,
				"period_from":         report.StringPeriodFrom,
				"period_to":           report.StringPeriodTo,
				"license_agreement":   merchant.AgreementNumber,
				"status":              report.Status,
				"merchant_greeting":   merchant.GetOwnerName(),
				"royalty_reports_url": s.cfg.GetRoyaltyReportsUrl(),
				"current_year":        time.Now().UTC().Format("2006"),
			},
			To: merchant.GetOwnerEmail(),
		}

		err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, payload, amqp.Table{})

		if err != nil {
			zap.L().Error(
				"Publication message about merchant new royalty report to queue failed",
				zap.Error(err),
				zap.Any("report", report),
			)
		}
	}

	msg := map[string]interface{}{"id": report.Id, "code": "rr00001", "message": pkg.EmailRoyaltyReportMessage}
	err = s.centrifugoDashboard.Publish(ctx, fmt.Sprintf(s.cfg.CentrifugoMerchantChannel, report.MerchantId), msg)

	if err != nil {
		zap.L().Error(
			"[Centrifugo] Send merchant notification about new royalty report failed",
			zap.Error(err),
			zap.Any("msg", msg),
		)
	}

	return
}

func (s *Service) royaltyReportSetPaid(ctx context.Context, reportIds []string, payoutDocumentId, ip, source string) (err error) {
	_, err = primitive.ObjectIDFromHex(payoutDocumentId)

	if err != nil {
		return royaltyReportErrorPayoutDocumentIdInvalid
	}

	for _, id := range reportIds {
		rr, err := s.royaltyReportRepository.GetById(ctx, id)
		if err != nil {
			return err
		}

		rr.PayoutDocumentId = payoutDocumentId
		rr.Status = billingpb.RoyaltyReportStatusPaid
		rr.PayoutDate = ptypes.TimestampNow()

		err = s.royaltyReportRepository.Update(ctx, rr, ip, source)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) royaltyReportSetPayoutDocumentId(ctx context.Context, reportIds []string, payoutDocumentId, ip, source string) error {
	_, err := primitive.ObjectIDFromHex(payoutDocumentId)

	if err != nil {
		return royaltyReportErrorPayoutDocumentIdInvalid
	}

	for _, id := range reportIds {
		rr, err := s.royaltyReportRepository.GetById(ctx, id)
		if err != nil {
			return err
		}

		rr.PayoutDocumentId = payoutDocumentId
		rr.Status = billingpb.RoyaltyReportStatusWaitForPayment

		err = s.royaltyReportRepository.Update(ctx, rr, ip, source)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) royaltyReportChangedEmail(ctx context.Context, report *billingpb.RoyaltyReport) error {
	merchant, err := s.merchantRepository.GetById(ctx, report.MerchantId)

	if err != nil {
		zap.L().Error(
			"get merchant for send email about royalty report document was changed failed",
			zap.Error(err),
			zap.Any("merchant_id", report.MerchantId),
			zap.Any("report_id", report.Id),
		)
		return royaltyReportErrorMerchantNotFound
	}

	periodFrom, err := ptypes.Timestamp(report.PeriodFrom)

	if err != nil {
		zap.L().Error(
			pkg.ErrorTimeConversion,
			zap.Any(pkg.ErrorTimeConversionMethod, "ptypes.Timestamp"),
			zap.Any(pkg.ErrorTimeConversionValue, report.PeriodFrom),
			zap.Error(err),
		)
		return royaltyReportEntryErrorUnknown
	}

	periodTo, err := ptypes.Timestamp(report.PeriodTo)

	if err != nil {
		zap.L().Error(
			pkg.ErrorTimeConversion,
			zap.Any(pkg.ErrorTimeConversionMethod, "ptypes.Timestamp"),
			zap.Any(pkg.ErrorTimeConversionValue, report.PeriodTo),
			zap.Error(err),
		)
		return royaltyReportEntryErrorUnknown
	}

	payload := &postmarkpb.Payload{
		TemplateAlias: s.cfg.EmailTemplates.UpdateRoyaltyReport,
		TemplateModel: map[string]string{
			"merchant_id":         merchant.Id,
			"royalty_report_id":   report.Id,
			"period_from":         periodFrom.Format("2006-01-02"),
			"period_to":           periodTo.Format("2006-01-02"),
			"license_agreement":   merchant.AgreementNumber,
			"merchant_greeting":   merchant.GetOwnerName(),
			"royalty_reports_url": s.cfg.GetRoyaltyReportsUrl(),
			"current_year":        time.Now().UTC().Format("2006"),
		},
		To: merchant.GetOwnerEmail(),
	}

	err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, payload, amqp.Table{})

	if err != nil {
		zap.L().Error(
			"Publication message about merchant royalty report document was changed to queue failed",
			zap.Error(err),
			zap.Any("report", report),
		)
		return royaltyReportEntryErrorUnknown
	}

	return nil
}

func (s *Service) RoyaltyReportFinanceDone(
	ctx context.Context,
	req *billingpb.ReportFinanceDoneRequest,
	res *billingpb.EmptyResponseWithStatus,
) error {
	royaltyReport, err := s.royaltyReportRepository.GetById(ctx, req.RoyaltyReportId)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = royaltyReportEntryErrorUnknown

		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = royaltyReportErrorReportNotFound
		}

		return nil
	}

	attachment := &postmarkpb.PayloadAttachment{
		Name:        req.FileName,
		Content:     base64.StdEncoding.EncodeToString(req.FileContent),
		ContentType: http.DetectContentType(req.FileContent),
	}
	attachments, err := s.royaltyReportRepository.SetRoyaltyReportFinanceItem(royaltyReport.Id, attachment)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = royaltyReportEntryErrorUnknown
		return nil
	}

	if len(attachments) < financeReportsCountForSend {
		res.Status = billingpb.ResponseStatusOk
		return nil
	}

	err = s.royaltyReportRepository.RemoveRoyaltyReportFinanceItems(royaltyReport.Id)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = royaltyReportEntryErrorUnknown
		return nil
	}

	action := "accept"
	disputeReason := ""

	if royaltyReport.Status == billingpb.RoyaltyReportStatusDispute {
		action = "start dispute for"
		disputeReason = royaltyReport.DisputeReason
	}

	payload := &postmarkpb.Payload{
		TemplateAlias: s.cfg.EmailTemplates.RoyaltyReportFinancier,
		TemplateModel: map[string]string{
			"merchant_id":            req.MerchantId,
			"merchant_name":          req.MerchantName,
			"royalty_report_id":      royaltyReport.Id,
			"period_from":            req.PeriodFrom,
			"period_to":              req.PeriodTo,
			"license_agreement":      req.LicenseAgreementNumber,
			"status":                 royaltyReport.Status,
			"operating_company_name": req.OperatingCompanyName,
			"action":                 action,
			"dispute_reason":         disputeReason,
			"royalty_report_url":     s.cfg.GetRoyaltyReportAdminUrl(royaltyReport.Id),
			"current_year":           time.Now().UTC().Format("2006"),
		},
		To:          s.cfg.EmailNotificationFinancierRecipient,
		Attachments: attachments,
	}

	err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, payload, amqp.Table{})

	if err != nil {
		zap.L().Error(
			"Publication message to postmark broker failed",
			zap.Error(err),
			zap.Any("payload", payload),
		)
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = royaltyReportEntryErrorUnknown
		return nil
	}

	res.Status = billingpb.ResponseStatusOk
	return nil
}

func (s *Service) onRoyaltyReportStatusChanged(
	ctx context.Context,
	royaltyReport *billingpb.RoyaltyReport,
) error {
	merchant, err := s.merchantRepository.GetById(ctx, royaltyReport.MerchantId)

	if err != nil {
		return merchantErrorNotFound
	}

	params := []byte(`{"` + reporterpb.ParamsFieldId + `": "` + royaltyReport.Id + `"}`)
	req := &reporterpb.ReportFile{
		UserId:     merchant.User.Id,
		MerchantId: merchant.Id,
		ReportType: reporterpb.ReportTypeRoyaltyAccountant,
		FileType:   reporterpb.OutputExtensionXlsx,
		Params:     params,
	}
	err = s.reporterServiceCreateFile(ctx, req)

	if err != nil {
		return err
	}

	req.ReportType = reporterpb.ReportTypeRoyaltyTransactionsAccountant
	return s.reporterServiceCreateFile(ctx, req)
}

func (s *Service) markOrdersIncludedToRoyaltyReport(
	ctx context.Context,
	royaltyReportId string,
	orderIds []primitive.ObjectID,
) error {
	err := s.orderRepository.IncludeOrdersToRoyaltyReport(ctx, royaltyReportId, orderIds)

	if err != nil {
		return err
	}

	orderIdsString := make([]string, 0, len(orderIds))

	for _, id := range orderIds {
		orderIdsString = append(orderIdsString, id.Hex())
	}

	return s.updateOrderView(ctx, orderIdsString)
}

func (h *royaltyHandler) buildMerchantRoyaltyReportRoundedAmounts(
	ctx context.Context, merchant *billingpb.Merchant, hasExistsReportId bool,
) (*billingpb.RoyaltyReport, []primitive.ObjectID, error) {
	merchantPayoutCurrency := merchant.GetPayoutCurrency()
	summaryItems, summaryTotal, ordersIds, err := h.orderViewRepository.GetRoyaltySummaryRoundedAmounts(
		ctx,
		merchant.Id,
		merchantPayoutCurrency,
		h.from,
		h.to,
		hasExistsReportId,
	)

	if err != nil {
		return nil, nil, err
	}

	corrections, correctionsTotal, err := h.getRoyaltyReportCorrections(ctx, merchant.Id, merchantPayoutCurrency)
	if err != nil {
		return nil, nil, err
	}

	reserves, reservesTotal, err := h.getRoyaltyReportRollingReserves(ctx, merchant.Id, merchantPayoutCurrency)
	if err != nil {
		return nil, nil, err
	}

	report := &billingpb.RoyaltyReport{
		Id:                 primitive.NewObjectID().Hex(),
		MerchantId:         merchant.Id,
		OperatingCompanyId: merchant.OperatingCompanyId,
		Currency:           merchant.GetPayoutCurrency(),
		Status:             billingpb.RoyaltyReportStatusPending,
		CreatedAt:          ptypes.TimestampNow(),
		UpdatedAt:          ptypes.TimestampNow(),
		Totals: &billingpb.RoyaltyReportTotals{
			TransactionsCount:    summaryTotal.TotalTransactions,
			FeeAmount:            summaryTotal.TotalFees,
			VatAmount:            summaryTotal.TotalVat,
			PayoutAmount:         summaryTotal.PayoutAmount,
			CorrectionAmount:     correctionsTotal,
			RollingReserveAmount: reservesTotal,
		},
		Summary: &billingpb.RoyaltyReportSummary{
			ProductsItems:   summaryItems,
			ProductsTotal:   summaryTotal,
			Corrections:     corrections,
			RollingReserves: reserves,
		},
	}

	report.PeriodFrom, err = ptypes.TimestampProto(h.from)
	if err != nil {
		return nil, nil, err
	}
	report.StringPeriodFrom = h.from.Format("2006-01-02")

	report.PeriodTo, err = ptypes.TimestampProto(h.to)
	if err != nil {
		return nil, nil, err
	}

	// temporary f*cking magic, will be deleted after switch royalty reports generation to UTC time
	to, err := ptypes.Timestamp(report.PeriodTo)
	if err != nil {
		return nil, nil, err
	}
	to = now.New(to).EndOfDay()
	report.StringPeriodTo = to.Format("2006-01-02")

	report.AcceptExpireAt, err = ptypes.TimestampProto(time.Now().Add(time.Duration(h.cfg.RoyaltyReportAcceptTimeout) * time.Second))
	if err != nil {
		return nil, nil, err
	}

	return report, ordersIds, nil
}
