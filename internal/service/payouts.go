package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/golang/protobuf/ptypes"
	"github.com/jinzhu/now"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/postmarkpb"
	"github.com/paysuper/paysuper-proto/go/reporterpb"
	tools "github.com/paysuper/paysuper-tools/number"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"math"
	"mime"
	"net/http"
	"path/filepath"
	"sort"
	"time"
)

const (
	payoutChangeSourceMerchant = "merchant"
	payoutChangeSourceAdmin    = "admin"

	payoutArrivalInDays = 5
)

var (
	errorPayoutSourcesNotFound         = errors.NewBillingServerErrorMsg("po000001", "no source documents found for payout")
	errorPayoutSourcesPending          = errors.NewBillingServerErrorMsg("po000002", "you have at least one royalty report waiting for acceptance")
	errorPayoutSourcesDispute          = errors.NewBillingServerErrorMsg("po000003", "you have at least one unclosed dispute in your royalty reports")
	errorPayoutNotFound                = errors.NewBillingServerErrorMsg("po000004", "payout document not found")
	errorPayoutAmountInvalid           = errors.NewBillingServerErrorMsg("po000005", "payout amount is invalid")
	errorPayoutUpdateBalance           = errors.NewBillingServerErrorMsg("po000008", "balance update failed")
	errorPayoutBalanceError            = errors.NewBillingServerErrorMsg("po000009", "getting balance failed")
	errorPayoutNotEnoughBalance        = errors.NewBillingServerErrorMsg("po000010", "not enough balance for payout")
	errorPayoutUpdateRoyaltyReports    = errors.NewBillingServerErrorMsg("po000012", "royalty reports update failed")
	errorPayoutStatusChangeIsForbidden = errors.NewBillingServerErrorMsg("po000014", "status change is forbidden")
	errorPayoutManualPayoutsDisabled   = errors.NewBillingServerErrorMsg("po000015", "manual payouts disabled")
	errorPayoutAutoPayoutsDisabled     = errors.NewBillingServerErrorMsg("po000016", "auto payouts disabled")
	errorPayoutAutoPayoutsWithErrors   = errors.NewBillingServerErrorMsg("po000017", "auto payouts creation finished with errors")
	errorPayoutRequireFailureFields    = errors.NewBillingServerErrorMsg("po000018", "fields failure_code and failure_message is required when status changing to failure")
	errorPayoutTarrifNotFound          = errors.NewBillingServerErrorMsg("po000019", "not found minimal tarrif for payout currency")
	errorPayoutTarrifMinimal           = errors.NewBillingServerErrorMsg("po000020", "minimal payout should be greater")

	statusForUpdateBalance = map[string]bool{
		pkg.PayoutDocumentStatusPending: true,
		pkg.PayoutDocumentStatusPaid:    true,
		pkg.PayoutDocumentStatusSkip:    true,
	}

	statusForBecomePaid = map[string]bool{
		pkg.PayoutDocumentStatusPaid: true,
	}

	statusForBecomeFailed = map[string]bool{
		pkg.PayoutDocumentStatusFailed:   true,
		pkg.PayoutDocumentStatusCanceled: true,
		pkg.PayoutDocumentStatusSkip:     true,
	}
)

func (s *Service) CreatePayoutDocument(
	ctx context.Context,
	req *billingpb.CreatePayoutDocumentRequest,
	res *billingpb.CreatePayoutDocumentResponse,
) error {

	merchant, err := s.merchantRepository.GetById(ctx, req.MerchantId)
	if err != nil {
		return merchantErrorNotFound
	}

	if merchant.ManualPayoutsEnabled == req.IsAutoGeneration {
		res.Status = billingpb.ResponseStatusBadData
		if req.IsAutoGeneration {
			res.Message = errorPayoutAutoPayoutsDisabled
		} else {
			res.Message = errorPayoutManualPayoutsDisabled
		}
		return nil
	}

	return s.createPayoutDocument(ctx, merchant, req, res)
}

func (s *Service) createPayoutDocument(
	ctx context.Context,
	merchant *billingpb.Merchant,
	req *billingpb.CreatePayoutDocumentRequest,
	res *billingpb.CreatePayoutDocumentResponse,
) error {
	arrivalDate, err := ptypes.TimestampProto(now.EndOfDay().Add(time.Hour * 24 * payoutArrivalInDays))
	if err != nil {
		return err
	}

	autoincrementId, err := s.autoincrementRepository.GatPayoutAutoincrementId(ctx)
	if err != nil {
		return err
	}

	pd := &billingpb.PayoutDocument{
		Id:                      primitive.NewObjectID().Hex(),
		Status:                  pkg.PayoutDocumentStatusPending,
		SourceId:                []string{},
		Description:             req.Description,
		CreatedAt:               ptypes.TimestampNow(),
		UpdatedAt:               ptypes.TimestampNow(),
		ArrivalDate:             arrivalDate,
		MerchantId:              merchant.Id,
		Destination:             merchant.Banking,
		Company:                 merchant.Company,
		MerchantAgreementNumber: merchant.AgreementNumber,
		OperatingCompanyId:      merchant.OperatingCompanyId,
		AutoincrementId:         autoincrementId,
	}

	reports, err := s.getPayoutDocumentSources(ctx, merchant)

	if err != nil {
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = e
			return nil
		}
		return err
	}

	pd.Currency = reports[0].Currency

	times := make([]time.Time, 0)
	stringTimes := make([]string, 0)

	for _, r := range reports {
		pd.TotalFees += r.Totals.PayoutAmount + r.Totals.CorrectionAmount
		pd.Balance += r.Totals.PayoutAmount + r.Totals.CorrectionAmount - r.Totals.RollingReserveAmount
		pd.TotalTransactions += r.Totals.TransactionsCount
		pd.SourceId = append(pd.SourceId, r.Id)

		from, err := ptypes.Timestamp(r.PeriodFrom)

		if err != nil {
			zap.L().Error(
				"Time conversion error",
				zap.Error(err),
			)
			return err
		}

		to, err := ptypes.Timestamp(r.PeriodTo)
		if err != nil {
			zap.L().Error(
				"Payout source time conversion error",
				zap.Error(err),
			)
			return err
		}
		times = append(times, from, to)
		stringTimes = append(stringTimes, r.StringPeriodFrom, r.StringPeriodTo)
	}

	currency := merchant.GetPayoutCurrency()
	var minimal float32
	var ok bool

	if merchant.Tariff == nil {
		zap.L().Error("merchant does not have tariff", zap.String("merchant_id", merchant.Id))
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPayoutTarrifNotFound
		return nil
	}

	if minimal, ok = merchant.Tariff.MinimalPayout[currency]; !ok {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPayoutTarrifNotFound
		return nil
	}

	pd.TotalFees = math.Round(pd.TotalFees*100) / 100
	pd.Balance = math.Round(pd.Balance*100) / 100

	if pd.Balance < float64(minimal) {
		if req.IsAutoGeneration {
			zap.L().Info("balance is less than minimal in tariff for merchant currency. skipping auto-generation payout",
				zap.String("merchant_id", merchant.Id),
				zap.Float32("minimal", minimal),
				zap.Float64("amount", pd.Balance),
				zap.String("currency", currency))
			res.Status = billingpb.ResponseStatusOk
			return nil
		}
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPayoutTarrifMinimal
		return nil
	}

	if pd.Balance <= 0 {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPayoutAmountInvalid
		return nil
	}

	balance, err := s.getMerchantBalance(ctx, merchant.Id)
	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorPayoutBalanceError
		return nil
	}

	balanceNet := tools.ToPrecise(balance.Debit - balance.Credit)

	if pd.Balance > balanceNet {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPayoutNotEnoughBalance
		return nil
	}

	if pd.Balance < merchant.MinPayoutAmount {
		pd.Status = pkg.PayoutDocumentStatusSkip
	}

	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})

	sort.Strings(stringTimes)

	from := times[0]
	to := times[len(times)-1]

	pd.PeriodFrom, err = ptypes.TimestampProto(from)
	if err != nil {
		zap.L().Error(
			"Payout PeriodFrom time conversion error",
			zap.Error(err),
		)
		return err
	}
	pd.PeriodTo, err = ptypes.TimestampProto(to)
	if err != nil {
		zap.L().Error(
			"Payout PeriodTo time conversion error",
			zap.Error(err),
		)
		return err
	}

	pd.StringPeriodFrom = stringTimes[0]
	pd.StringPeriodTo = stringTimes[len(stringTimes)-1]

	err = s.payoutRepository.Insert(ctx, pd, req.Ip, payoutChangeSourceMerchant)

	if err != nil {
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = e
			return nil
		}
		return err
	}

	err = s.royaltyReportSetPayoutDocumentId(ctx, pd.SourceId, pd.Id, req.Ip, req.Initiator)

	if err != nil {
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = e
			return nil
		}
		return err
	}

	_, err = s.updateMerchantBalance(ctx, merchant.Id)
	if err != nil {
		e, ok := err.(*billingpb.ResponseErrorMessage)

		if !ok {
			e = merchantErrorUnknown
		}

		res.Status = billingpb.ResponseStatusSystemError
		res.Message = e
		return nil
	}

	err = s.renderPayoutDocument(ctx, pd, merchant)
	if err != nil {
		return err
	}

	res.Items = append(res.Items, pd)

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetPayoutDocument(
	ctx context.Context,
	req *billingpb.GetPayoutDocumentRequest,
	res *billingpb.PayoutDocumentResponse,
) (err error) {
	res.Item, err = s.payoutRepository.GetByIdMerchantId(ctx, req.PayoutDocumentId, req.MerchantId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = errorPayoutNotFound
			return nil
		}
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = e
			return nil
		}
		return err
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetPayoutDocumentRoyaltyReports(
	ctx context.Context,
	req *billingpb.GetPayoutDocumentRequest,
	res *billingpb.ListRoyaltyReportsResponse,
) error {
	pd, err := s.payoutRepository.GetByIdMerchantId(ctx, req.PayoutDocumentId, req.MerchantId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = errorPayoutNotFound
			return nil
		}
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = e
			return nil
		}
		return err
	}

	res.Data = &billingpb.RoyaltyReportsPaginate{}
	res.Data.Items, err = s.royaltyReportRepository.GetByPayoutId(ctx, pd.Id)

	if err != nil {
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = e
			return nil
		}
		return err
	}

	res.Data.Count = int64(len(res.Data.Items))
	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) AutoCreatePayoutDocuments(
	ctx context.Context,
	_ *billingpb.EmptyRequest,
	_ *billingpb.EmptyResponse,
) error {
	zap.L().Info("start auto-creation of payout documents")

	merchants, err := s.merchantRepository.GetMerchantsWithAutoPayouts(ctx)
	if err != nil {
		zap.L().Error("GetMerchantsWithAutoPayouts failed", zap.Error(err))
		return err
	}

	req1 := &billingpb.CreatePayoutDocumentRequest{
		Ip:               "0.0.0.0",
		Initiator:        pkg.RoyaltyReportChangeSourceAuto,
		IsAutoGeneration: true,
	}
	res := &billingpb.CreatePayoutDocumentResponse{}

	wasErrors := false

	for _, m := range merchants {
		req1.MerchantId = m.Id
		err = s.createPayoutDocument(ctx, m, req1, res)

		if err != nil {
			if err == errorPayoutSourcesNotFound {
				continue
			}
			zap.L().Error(
				"auto createPayoutDocument failed with error",
				zap.Error(err),
				zap.String("merchantId", m.Id),
			)
			wasErrors = true
			continue
		}
		if res.Status != billingpb.ResponseStatusOk {
			if res.Message == errorPayoutAmountInvalid || res.Message == errorPayoutSourcesNotFound {
				continue
			}
			zap.L().Error(
				"auto createPayoutDocument failed in response",
				zap.Int32("code", res.Status),
				zap.Any("message", res.Message),
				zap.String("merchantId", m.Id),
			)
			wasErrors = true
			continue
		}
	}

	if wasErrors {
		zap.L().Warn("auto-creation of payout documents finished with errors")

		return errorPayoutAutoPayoutsWithErrors
	}

	zap.L().Info("auto-creation of payout documents finished")

	return nil
}

func (s *Service) renderPayoutDocument(
	ctx context.Context,
	pd *billingpb.PayoutDocument,
	merchant *billingpb.Merchant,
) error {
	params, err := json.Marshal(map[string]interface{}{reporterpb.ParamsFieldId: pd.Id})
	if err != nil {
		zap.L().Error(
			"Unable to marshal the params of payout for the reporting service.",
			zap.Error(err),
		)
		return err
	}

	req := &reporterpb.ReportFile{
		UserId:           merchant.User.Id,
		MerchantId:       merchant.Id,
		ReportType:       reporterpb.ReportTypePayout,
		FileType:         reporterpb.OutputExtensionPdf,
		Params:           params,
		SendNotification: merchant.ManualPayoutsEnabled,
	}
	err = s.reporterServiceCreateFile(ctx, req)

	if err != nil {
		return err
	}

	req.ReportType = reporterpb.ReportTypePayoutFinance
	req.FileType = reporterpb.OutputExtensionXlsx
	req.SendNotification = false
	err = s.reporterServiceCreateFile(ctx, req)

	if err != nil {
		return err
	}

	return nil
}

func (s *Service) UpdatePayoutDocument(
	ctx context.Context,
	req *billingpb.UpdatePayoutDocumentRequest,
	res *billingpb.PayoutDocumentResponse,
) error {
	pd, err := s.payoutRepository.GetById(ctx, req.PayoutDocumentId)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = errorPayoutNotFound
			return nil
		}
		return err
	}

	isChanged := false
	needBalanceUpdate := false

	_, isReqStatusForBecomePaid := statusForBecomePaid[req.Status]
	becomePaid := isReqStatusForBecomePaid && pd.Status != req.Status

	_, isReqStatusForBecomeFailed := statusForBecomeFailed[req.Status]
	_, isPayoutStatusForBecomeFailed := statusForBecomeFailed[pd.Status]
	becomeFailed := isReqStatusForBecomeFailed && !isPayoutStatusForBecomeFailed

	if req.Status != "" && pd.Status != req.Status {
		if pd.Status != pkg.PayoutDocumentStatusPending {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = errorPayoutStatusChangeIsForbidden
			return nil
		}

		if req.Status == pkg.PayoutDocumentStatusFailed && (req.FailureCode == "" || req.FailureMessage == "") {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = errorPayoutRequireFailureFields
			return nil
		}

		if req.Status == pkg.PayoutDocumentStatusPaid {
			pd.PaidAt = ptypes.TimestampNow()
		}

		isChanged = true
		pd.Status = req.Status
		if _, ok := statusForUpdateBalance[pd.Status]; ok {
			needBalanceUpdate = true
		}
	}

	if req.Transaction != "" && pd.Transaction != req.Transaction {
		isChanged = true
		pd.Transaction = req.Transaction
	}

	if req.FailureCode != "" && pd.FailureCode != req.FailureCode {
		isChanged = true
		pd.FailureCode = req.FailureCode
	}

	if req.FailureMessage != "" && pd.FailureMessage != req.FailureMessage {
		isChanged = true
		pd.FailureMessage = req.FailureMessage
	}

	if req.FailureTransaction != "" && pd.FailureTransaction != req.FailureTransaction {
		isChanged = true
		pd.FailureTransaction = req.FailureTransaction
	}

	if isChanged {
		err = s.payoutRepository.Update(ctx, pd, req.Ip, payoutChangeSourceAdmin)
		if err != nil {
			if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
				res.Status = billingpb.ResponseStatusSystemError
				res.Message = e
				return nil
			}
			return err
		}

		if becomePaid == true {
			err = s.royaltyReportSetPaid(ctx, pd.SourceId, pd.Id, req.Ip, pkg.RoyaltyReportChangeSourceAdmin)
			if err != nil {
				res.Status = billingpb.ResponseStatusSystemError
				res.Message = errorPayoutUpdateRoyaltyReports

				return nil
			}

		} else {
			if becomeFailed == true {
				err = s.royaltyReportUnsetPaid(ctx, pd.SourceId, req.Ip, pkg.RoyaltyReportChangeSourceAdmin)
				if err != nil {
					res.Status = billingpb.ResponseStatusSystemError
					res.Message = errorPayoutUpdateRoyaltyReports

					return nil
				}
			}
		}

		res.Status = billingpb.ResponseStatusOk
	} else {
		res.Status = billingpb.ResponseStatusNotModified
	}

	if needBalanceUpdate == true {
		_, err = s.updateMerchantBalance(ctx, pd.MerchantId)
		if err != nil {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = errorPayoutUpdateBalance

			return nil
		}
	}

	res.Item = pd
	return nil
}

func (s *Service) royaltyReportUnsetPaid(ctx context.Context, reportIds []string, ip, source string) (err error) {
	for _, id := range reportIds {
		rr, err := s.royaltyReportRepository.GetById(ctx, id)
		if err != nil {
			return err
		}

		rr.PayoutDocumentId = ""
		rr.Status = billingpb.RoyaltyReportStatusAccepted
		rr.PayoutDate = nil

		err = s.royaltyReportRepository.Update(ctx, rr, ip, source)
		if err != nil {
			return err
		}
	}
	return
}

func (s *Service) GetPayoutDocuments(
	ctx context.Context,
	req *billingpb.GetPayoutDocumentsRequest,
	res *billingpb.GetPayoutDocumentsResponse,
) error {
	res.Status = billingpb.ResponseStatusOk
	count, err := s.payoutRepository.FindCount(ctx, req)

	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	if count == 0 {
		res.Status = billingpb.ResponseStatusOk
		res.Data = &billingpb.PayoutDocumentsPaginate{
			Count: 0,
			Items: nil,
		}
		return nil
	}

	pds, err := s.payoutRepository.Find(ctx, req)

	if err != nil {
		return err
	}

	res.Status = billingpb.ResponseStatusOk
	res.Data = &billingpb.PayoutDocumentsPaginate{
		Count: int32(count),
		Items: pds,
	}
	return nil
}

func (s *Service) PayoutDocumentPdfUploaded(
	ctx context.Context,
	req *billingpb.PayoutDocumentPdfUploadedRequest,
	res *billingpb.PayoutDocumentPdfUploadedResponse,
) error {
	pd, err := s.payoutRepository.GetById(ctx, req.PayoutId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = errorPayoutNotFound
			return nil
		}
		return err
	}

	merchant, err := s.merchantRepository.GetById(ctx, pd.MerchantId)

	if err != nil {
		return merchantErrorNotFound
	}

	if merchant.HasAuthorizedEmail() == false {
		zap.L().Warn("Merchant has no authorized email", zap.String("merchant_id", merchant.Id))
		res.Status = billingpb.ResponseStatusOk
		return nil
	}

	content := base64.StdEncoding.EncodeToString(req.Content)
	contentType := mime.TypeByExtension(filepath.Ext(req.Filename))

	operatingCompany, err := s.operatingCompanyRepository.GetById(ctx, pd.OperatingCompanyId)

	if err != nil {
		zap.L().Error("Operating company not found", zap.Error(err), zap.String("operating_company_id", pd.OperatingCompanyId))
		return err
	}

	payload := &postmarkpb.Payload{
		TemplateAlias: s.cfg.EmailTemplates.NewPayout,
		TemplateModel: map[string]string{
			"merchant_id":            merchant.Id,
			"payout_id":              pd.Id,
			"period_from":            pd.StringPeriodFrom,
			"period_to":              pd.StringPeriodTo,
			"license_agreement":      merchant.AgreementNumber,
			"status":                 pd.Status,
			"merchant_greeting":      merchant.GetOwnerName(),
			"payouts_url":            s.cfg.GetPayoutsUrl(),
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
			zap.Any("payout document", pd),
		)
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) getPayoutDocumentSources(
	ctx context.Context,
	merchant *billingpb.Merchant,
) ([]*billingpb.RoyaltyReport, error) {
	result, err := s.royaltyReportRepository.GetNonPayoutReports(ctx, merchant.Id, merchant.GetPayoutCurrency())

	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	if err == mongo.ErrNoDocuments || result == nil || len(result) == 0 {
		return nil, errorPayoutSourcesNotFound
	}

	for _, v := range result {
		if v.Status == billingpb.RoyaltyReportStatusPending {
			return nil, errorPayoutSourcesPending
		}
		if v.Status == billingpb.RoyaltyReportStatusDispute {
			return nil, errorPayoutSourcesDispute
		}
	}

	return result, nil
}

func (s *Service) PayoutFinanceDone(
	ctx context.Context,
	req *billingpb.ReportFinanceDoneRequest,
	res *billingpb.EmptyResponseWithStatus,
) error {
	payout, err := s.payoutRepository.GetById(ctx, req.PayoutId)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorPayoutNotFound

		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = errorPayoutNotFound
		}

		return nil
	}

	payload := &postmarkpb.Payload{
		TemplateAlias: s.cfg.EmailTemplates.PayoutInvoiceFinancier,
		TemplateModel: map[string]string{
			"merchant_id":            req.MerchantId,
			"merchant_name":          req.MerchantName,
			"payout_id":              payout.Id,
			"period_from":            req.PeriodFrom,
			"period_to":              req.PeriodTo,
			"license_agreement":      req.LicenseAgreementNumber,
			"status":                 payout.Status,
			"operating_company_name": req.OperatingCompanyName,
			"payout_url":             s.cfg.GetSystemPayoutUrl(payout.Id),
			"current_year":           time.Now().UTC().Format("2006"),
		},
		To: s.cfg.EmailNotificationFinancierRecipient,
		Attachments: []*postmarkpb.PayloadAttachment{
			{
				Name:        req.FileName,
				Content:     base64.StdEncoding.EncodeToString(req.FileContent),
				ContentType: http.DetectContentType(req.FileContent),
			},
		},
	}

	err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, payload, amqp.Table{})

	if err != nil {
		zap.L().Error(
			"Publication message to postmark broker failed",
			zap.Error(err),
			zap.Any("payload", payload),
		)
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorPayoutNotFound
		return nil
	}

	res.Status = billingpb.ResponseStatusOk
	return nil
}
