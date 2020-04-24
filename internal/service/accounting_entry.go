package service

import (
	"context"
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-billing-server/internal/repository"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/currenciespb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	tools "github.com/paysuper/paysuper-tools/number"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"strconv"
	"time"
)

const (
	errorFieldService     = "service"
	errorFieldMethod      = "method"
	errorFieldEntry       = "entry"
	errorFieldRequest     = "request"
	errorFieldEntrySource = "source_id"
	errorFieldStatus      = "status"
	errorFieldMessage     = "message"

	accountingEventTypePayment          = "payment"
	accountingEventTypeRefund           = "refund"
	accountingEventTypeManualCorrection = "manual-correction"
)

var (
	accountingEntryErrorOrderNotFound              = newBillingServerErrorMsg("ae00001", "order not found for creating accounting entry")
	accountingEntryErrorRefundNotFound             = newBillingServerErrorMsg("ae00002", "refund not found for creating accounting entry")
	accountingEntryErrorMerchantNotFound           = newBillingServerErrorMsg("ae00003", "merchant not found for creating accounting entry")
	accountingEntryErrorMerchantCommissionNotFound = newBillingServerErrorMsg("ae00004", "commission to merchant and payment method not found")
	accountingEntryErrorExchangeFailed             = newBillingServerErrorMsg("ae00005", "currency exchange failed")
	accountingEntryErrorUnknownEntry               = newBillingServerErrorMsg("ae00006", "unknown accounting entry type")
	accountingEntryErrorUnknown                    = newBillingServerErrorMsg("ae00007", "unknown error. try request later")
	accountingEntryErrorRefundExceedsOrderAmount   = newBillingServerErrorMsg("ae00008", "refund exceeds order amount")
	accountingEntryErrorCountryNotFound            = newBillingServerErrorMsg("ae00009", "country not found")
	accountingEntryUnknownEvent                    = newBillingServerErrorMsg("ae00010", "accounting unknown event")
	accountingEntryErrorUnknownSourceType          = newBillingServerErrorMsg("ae00011", "unknown accounting entry source type")
	accountingEntryErrorInvalidSourceId            = newBillingServerErrorMsg("ae00012", "accounting entry invalid source id")
	accountingEntryErrorSystemCommissionNotFound   = newBillingServerErrorMsg("ae00013", "system commission for payment method not found")
	accountingEntryAlreadyCreated                  = newBillingServerErrorMsg("ae00014", "accounting entries already created")
	accountingEntryBalanceUpdateFailed             = newBillingServerErrorMsg("ae00015", "balance update failed after create accounting entry")
	accountingEntryOriginalTaxNotFound             = newBillingServerErrorMsg("ae00016", "real_tax_fee entry from original order not found, refund processing failed")
	accountingEntryVatCurrencyNotSet               = newBillingServerErrorMsg("ae00017", "vat currency not set")

	availableAccountingEntries = map[string]bool{
		pkg.AccountingEntryTypeRealGrossRevenue:                    true,
		pkg.AccountingEntryTypePsGrossRevenueFx:                    true,
		pkg.AccountingEntryTypeRealTaxFee:                          true,
		pkg.AccountingEntryTypeRealTaxFeeTotal:                     true,
		pkg.AccountingEntryTypeCentralBankTaxFee:                   true,
		pkg.AccountingEntryTypePsGrossRevenueFxTaxFee:              true,
		pkg.AccountingEntryTypePsGrossRevenueFxProfit:              true,
		pkg.AccountingEntryTypeMerchantGrossRevenue:                true,
		pkg.AccountingEntryTypeMerchantTaxFeeCostValue:             true,
		pkg.AccountingEntryTypeMerchantTaxFeeCentralBankFx:         true,
		pkg.AccountingEntryTypeMerchantTaxFee:                      true,
		pkg.AccountingEntryTypePsMethodFee:                         true,
		pkg.AccountingEntryTypeMerchantMethodFee:                   true,
		pkg.AccountingEntryTypeMerchantMethodFeeCostValue:          true,
		pkg.AccountingEntryTypePsMarkupMerchantMethodFee:           true,
		pkg.AccountingEntryTypeMerchantMethodFixedFee:              true,
		pkg.AccountingEntryTypeRealMerchantMethodFixedFee:          true,
		pkg.AccountingEntryTypeMarkupMerchantMethodFixedFeeFx:      true,
		pkg.AccountingEntryTypeRealMerchantMethodFixedFeeCostValue: true,
		pkg.AccountingEntryTypePsMethodFixedFeeProfit:              true,
		pkg.AccountingEntryTypeMerchantPsFixedFee:                  true,
		pkg.AccountingEntryTypeRealMerchantPsFixedFee:              true,
		pkg.AccountingEntryTypeMarkupMerchantPsFixedFee:            true,
		pkg.AccountingEntryTypePsMethodProfit:                      true,
		pkg.AccountingEntryTypeMerchantNetRevenue:                  true,
		pkg.AccountingEntryTypePsProfitTotal:                       true,
		pkg.AccountingEntryTypeRealRefund:                          true,
		pkg.AccountingEntryTypeRealRefundTaxFee:                    true,
		pkg.AccountingEntryTypeRealRefundFee:                       true,
		pkg.AccountingEntryTypeRealRefundFixedFee:                  true,
		pkg.AccountingEntryTypeMerchantRefund:                      true,
		pkg.AccountingEntryTypePsMerchantRefundFx:                  true,
		pkg.AccountingEntryTypeMerchantRefundFee:                   true,
		pkg.AccountingEntryTypePsMarkupMerchantRefundFee:           true,
		pkg.AccountingEntryTypeMerchantRefundFixedFeeCostValue:     true,
		pkg.AccountingEntryTypeMerchantRefundFixedFee:              true,
		pkg.AccountingEntryTypePsMerchantRefundFixedFeeFx:          true,
		pkg.AccountingEntryTypePsMerchantRefundFixedFeeProfit:      true,
		pkg.AccountingEntryTypeReverseTaxFee:                       true,
		pkg.AccountingEntryTypeReverseTaxFeeDelta:                  true,
		pkg.AccountingEntryTypePsReverseTaxFeeDelta:                true,
		pkg.AccountingEntryTypeMerchantReverseTaxFee:               true,
		pkg.AccountingEntryTypeMerchantReverseRevenue:              true,
		pkg.AccountingEntryTypePsRefundProfit:                      true,
		pkg.AccountingEntryTypeMerchantRollingReserveCreate:        true,
		pkg.AccountingEntryTypeMerchantRollingReserveRelease:       true,
		pkg.AccountingEntryTypeMerchantRoyaltyCorrection:           true,
	}

	availableAccountingEntriesSourceTypes = map[string]bool{
		repository.CollectionOrder:    true,
		repository.CollectionRefund:   true,
		repository.CollectionMerchant: true,
	}

	rollingReserveAccountingEntries = map[string]bool{
		pkg.AccountingEntryTypeMerchantRollingReserveCreate:  true,
		pkg.AccountingEntryTypeMerchantRollingReserveRelease: true,
	}

	rollingReserveAccountingEntriesList = []string{
		pkg.AccountingEntryTypeMerchantRollingReserveCreate,
		pkg.AccountingEntryTypeMerchantRollingReserveRelease,
	}

	processableOrderStatus = []string{
		recurringpb.OrderPublicStatusProcessed,
		recurringpb.OrderPublicStatusRefunded,
		recurringpb.OrderPublicStatusChargeback,
	}
)

type accountingEntry struct {
	*Service
	ctx context.Context

	order             *billingpb.Order
	refund            *billingpb.Refund
	refundOrder       *billingpb.Order
	merchant          *billingpb.Merchant
	country           *billingpb.Country
	accountingEntries []*billingpb.AccountingEntry
	req               *billingpb.CreateAccountingEntryRequest
}

func (s *Service) CreateAccountingEntry(
	ctx context.Context,
	req *billingpb.CreateAccountingEntryRequest,
	rsp *billingpb.CreateAccountingEntryResponse,
) error {
	if _, ok := availableAccountingEntries[req.Type]; !ok {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = accountingEntryErrorUnknownEntry

		return nil
	}

	handler := &accountingEntry{Service: s, req: req, ctx: ctx}

	countryCode := ""
	_, err := primitive.ObjectIDFromHex(req.OrderId)

	if req.OrderId != "" && err == nil {
		order, err := s.getOrderById(ctx, req.OrderId)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = accountingEntryErrorOrderNotFound

			return nil
		}

		handler.order = order
		countryCode = order.GetCountry()
	}

	_, err = primitive.ObjectIDFromHex(req.RefundId)

	if req.RefundId != "" && err == nil {
		refund, err := s.refundRepository.GetById(ctx, req.RefundId)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = accountingEntryErrorRefundNotFound

			return nil
		}

		order, err := s.getOrderById(ctx, refund.OriginalOrder.Id)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = accountingEntryErrorOrderNotFound

			return nil
		}

		refundOrder, err := s.getOrderById(ctx, refund.CreatedOrderId)
		if err != nil {
			return err
		}

		handler.order = order
		handler.refund = refund
		handler.refundOrder = refundOrder
		countryCode = order.GetCountry()
	}

	if req.MerchantId != "" {
		merchant, err := s.merchantRepository.GetById(ctx, req.MerchantId)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = accountingEntryErrorMerchantNotFound

			return nil
		}

		handler.merchant = merchant
		countryCode = merchant.Company.Country
	}

	if countryCode == "" {
		countryCode = req.Country
	}

	country, err := s.country.GetByIsoCodeA2(ctx, countryCode)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = accountingEntryErrorCountryNotFound

		return nil
	}

	handler.country = country

	err = s.processEvent(handler, accountingEventTypeManualCorrection)
	if err != nil {
		zap.L().Error(
			pkg.MethodFinishedWithError,
			zap.String("method", "CreateAccountingEntry"),
			zap.Error(err),
			zap.Any("request", req),
		)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = accountingEntryErrorUnknown

		return nil
	}

	if _, ok := rollingReserveAccountingEntries[req.Type]; ok {
		_, err = s.updateMerchantBalance(ctx, handler.merchant.Id)
		if err != nil {
			rsp.Status = billingpb.ResponseStatusSystemError
			rsp.Message = accountingEntryBalanceUpdateFailed

			return nil
		}
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = handler.accountingEntries[0]

	return nil
}

func (s *Service) onPaymentNotify(ctx context.Context, order *billingpb.Order) error {
	if !helper.Contains(processableOrderStatus, order.GetPublicStatus()) {
		return nil
	}

	country, err := s.country.GetByIsoCodeA2(ctx, order.GetCountry())
	if err != nil {
		return err
	}

	merchant, err := s.merchantRepository.GetById(ctx, order.GetMerchantId())
	if err != nil {
		return merchantErrorNotFound
	}

	handler := &accountingEntry{
		Service:  s,
		order:    order,
		ctx:      ctx,
		country:  country,
		merchant: merchant,
	}

	return s.processEvent(handler, accountingEventTypePayment)
}

func (s *Service) onRefundNotify(ctx context.Context, refund *billingpb.Refund, order *billingpb.Order) error {
	if !helper.Contains(processableOrderStatus, order.GetPublicStatus()) {
		return nil
	}

	country, err := s.country.GetByIsoCodeA2(ctx, order.GetCountry())

	if err != nil {
		return err
	}

	refundOrder, err := s.getOrderById(ctx, refund.CreatedOrderId)

	if err != nil {
		return err
	}

	merchant, err := s.merchantRepository.GetById(ctx, refundOrder.GetMerchantId())
	if err != nil {
		return merchantErrorNotFound
	}

	handler := &accountingEntry{
		Service:     s,
		refund:      refund,
		order:       order,
		refundOrder: refundOrder,
		ctx:         ctx,
		country:     country,
		merchant:    merchant,
	}

	return s.processEvent(handler, accountingEventTypeRefund)
}

func (s *Service) processEvent(handler *accountingEntry, eventType string) error {
	var err error

	switch eventType {
	case accountingEventTypePayment:
		err = handler.processPaymentEvent()
		break

	case accountingEventTypeRefund:
		err = handler.processRefundEvent()
		break

	case accountingEventTypeManualCorrection:
		err = handler.processManualCorrectionEvent()
		break

	default:
		return accountingEntryUnknownEvent
	}

	if err != nil {
		return err
	}

	return handler.saveAccountingEntries(s.orderViewRepository, s.paylinkRepository, s.paylinkVisitsRepository)
}

func (h *accountingEntry) processManualCorrectionEvent() error {
	var err error

	entry := h.newEntry(h.req.Type)

	entry.Amount = h.req.Amount
	entry.Currency = h.req.Currency
	entry.Reason = h.req.Reason
	entry.Status = h.req.Status
	t := time.Unix(h.req.Date, 0)
	entry.CreatedAt, err = ptypes.TimestampProto(t)
	if err != nil {
		return err
	}

	if h.merchant != nil {
		entry.Source.Type = repository.CollectionMerchant
		entry.Source.Id = h.merchant.Id
		entry.MerchantId = h.merchant.Id
	}

	if err = h.addEntry(entry); err != nil {
		return err
	}

	return nil
}

func (h *accountingEntry) processPaymentEvent() error {
	var (
		amount float64
		err    error
	)

	ae, err := h.accountingRepository.GetByObjectSource(
		h.ctx,
		pkg.ObjectTypeBalanceTransaction,
		h.order.Id,
		repository.CollectionOrder,
	)

	if ae != nil {
		zap.L().Error(
			accountingEntryAlreadyCreated.Message,
			zap.Error(err),
			zap.String("source.type", repository.CollectionOrder),
			zap.String("source.id", h.order.Id),
			zap.Any("entries found", ae),
		)
		return accountingEntryAlreadyCreated
		// todo: is there must be an update of existing entry, instead of error?
	}

	// 1. realGrossRevenue
	realGrossRevenue := h.newEntry(pkg.AccountingEntryTypeRealGrossRevenue)
	realGrossRevenue.Amount, err = h.GetExchangePsCurrentCommon(h.order.ChargeCurrency, h.order.ChargeAmount)
	if err != nil {
		return err
	}
	realGrossRevenue.OriginalAmount = h.order.ChargeAmount
	realGrossRevenue.OriginalCurrency = h.order.ChargeCurrency
	if err = h.addEntry(realGrossRevenue); err != nil {
		return err
	}

	// 2. realTaxFee
	realTaxFee := h.newEntry(pkg.AccountingEntryTypeRealTaxFee)
	orderTaxAmount := h.order.GetTaxAmountInChargeCurrency()
	realTaxFee.Amount, err = h.GetExchangePsCurrentCommon(h.order.ChargeCurrency, orderTaxAmount)
	if err != nil {
		return err
	}
	realTaxFee.OriginalAmount = orderTaxAmount
	realTaxFee.OriginalCurrency = h.order.ChargeCurrency
	if err = h.addEntry(realTaxFee); err != nil {
		return err
	}

	// 3. centralBankTaxFee
	centralBankTaxFee := h.newEntry(pkg.AccountingEntryTypeCentralBankTaxFee)
	centralBankTaxFee.Amount = 0
	if err = h.addEntry(centralBankTaxFee); err != nil {
		return err
	}

	// 4. realTaxFeeTotal
	// calculated in order_view

	// 5. psGrossRevenueFx
	psGrossRevenueFx := h.newEntry(pkg.AccountingEntryTypePsGrossRevenueFx)
	amount, err = h.GetExchangePsCurrentMerchant(h.order.ChargeCurrency, h.order.ChargeAmount)
	if err != nil {
		return err
	}
	psGrossRevenueFx.Amount = amount - realGrossRevenue.Amount
	if err = h.addEntry(psGrossRevenueFx); err != nil {
		return err
	}

	// 6. psGrossRevenueFxTaxFee
	psGrossRevenueFxTaxFee := h.newEntry(pkg.AccountingEntryTypePsGrossRevenueFxTaxFee)
	psGrossRevenueFxTaxFee.Amount = tools.GetPercentPartFromAmount(psGrossRevenueFx.Amount, h.order.Tax.Rate)
	if err = h.addEntry(psGrossRevenueFxTaxFee); err != nil {
		return err
	}

	// 7. psGrossRevenueFxProfit
	// calculated in order_view

	// 8. merchantGrossRevenue
	merchantGrossRevenue := h.newEntry(pkg.AccountingEntryTypeMerchantGrossRevenue)
	merchantGrossRevenue.Amount = realGrossRevenue.Amount - psGrossRevenueFx.Amount
	// not store in DB - calculated in order_view, but used further in the method code

	// 9. merchantTaxFeeCostValue
	merchantTaxFeeCostValue := h.newEntry(pkg.AccountingEntryTypeMerchantTaxFeeCostValue)
	merchantTaxFeeCostValue.Amount = tools.GetPercentPartFromAmount(merchantGrossRevenue.Amount, h.order.Tax.Rate)
	if err = h.addEntry(merchantTaxFeeCostValue); err != nil {
		return err
	}

	// 10. merchantTaxFeeCentralBankFx
	merchantTaxFeeCentralBankFx := h.newEntry(pkg.AccountingEntryTypeMerchantTaxFeeCentralBankFx)
	if h.country.VatEnabled {
		amount, err = h.GetExchangeCbCurrentCommon(h.order.GetMerchantRoyaltyCurrency(), merchantTaxFeeCostValue.Amount)
		if err != nil {
			return err
		}
		amount, err = h.GetExchangeStockCurrentCommon(h.country.GetVatCurrencyCode(), amount)
		if err != nil {
			return err
		}
		merchantTaxFeeCentralBankFx.Amount = amount - merchantTaxFeeCostValue.Amount
		// merchantTaxFeeCentralBankFx amount can not be negative
		// @see https://protocolone.tpondemand.com/entity/196061
		if merchantTaxFeeCentralBankFx.Amount < 0 {
			merchantTaxFeeCentralBankFx.Amount = 0
		}
	}
	if err = h.addEntry(merchantTaxFeeCentralBankFx); err != nil {
		return err
	}

	// 11. merchantTaxFee
	// calculated in order_view

	paymentChannelCostMerchant, err := h.getPaymentChannelCostMerchant(realGrossRevenue.Amount)
	if err != nil {
		return err
	}

	paymentChannelCostSystem, err := h.getPaymentChannelCostSystem()
	if err != nil {
		return err
	}

	// 12. psMethodFee
	psMethodFee := h.newEntry(pkg.AccountingEntryTypePsMethodFee)
	psMethodFee.Amount = merchantGrossRevenue.Amount * paymentChannelCostMerchant.PsPercent
	if err = h.addEntry(psMethodFee); err != nil {
		return err
	}

	// 13. merchantMethodFee
	merchantMethodFee := h.newEntry(pkg.AccountingEntryTypeMerchantMethodFee)
	merchantMethodFee.Amount = merchantGrossRevenue.Amount * paymentChannelCostMerchant.MethodPercent
	if err = h.addEntry(merchantMethodFee); err != nil {
		return err
	}

	// 14. merchantMethodFeeCostValue
	merchantMethodFeeCostValue := h.newEntry(pkg.AccountingEntryTypeMerchantMethodFeeCostValue)
	merchantMethodFeeCostValue.Amount = realGrossRevenue.Amount * paymentChannelCostSystem.Percent
	if err = h.addEntry(merchantMethodFeeCostValue); err != nil {
		return err
	}

	// 15. psMarkupMerchantMethodFee
	// calculated in order_view

	// 16. merchantMethodFixedFee
	merchantMethodFixedFee := h.newEntry(pkg.AccountingEntryTypeMerchantMethodFixedFee)
	merchantMethodFixedFee.Amount, err = h.GetExchangePsCurrentMerchant(paymentChannelCostMerchant.MethodFixAmountCurrency, paymentChannelCostMerchant.MethodFixAmount)
	if err != nil {
		return err
	}

	if err = h.addEntry(merchantMethodFixedFee); err != nil {
		return err
	}

	// 17. realMerchantMethodFixedFee
	realMerchantMethodFixedFee := h.newEntry(pkg.AccountingEntryTypeRealMerchantMethodFixedFee)
	realMerchantMethodFixedFee.Amount, err = h.GetExchangePsCurrentCommon(paymentChannelCostMerchant.MethodFixAmountCurrency, paymentChannelCostMerchant.MethodFixAmount)
	if err != nil {
		return err
	}

	if err = h.addEntry(realMerchantMethodFixedFee); err != nil {
		return err
	}

	// 18. markupMerchantMethodFixedFeeFx
	// calculated in order_view

	// 19. realMerchantMethodFixedFeeCostValue
	realMerchantMethodFixedFeeCostValue := h.newEntry(pkg.AccountingEntryTypeRealMerchantMethodFixedFeeCostValue)
	realMerchantMethodFixedFeeCostValue.Amount, err = h.GetExchangePsCurrentCommon(paymentChannelCostSystem.FixAmountCurrency, paymentChannelCostSystem.FixAmount)
	if err != nil {
		return err
	}
	if err = h.addEntry(realMerchantMethodFixedFeeCostValue); err != nil {
		return err
	}

	// 20. psMethodFixedFeeProfit
	// calculated in order_view

	// 21. merchantPsFixedFee
	merchantPsFixedFee := h.newEntry(pkg.AccountingEntryTypeMerchantPsFixedFee)
	merchantPsFixedFee.Amount, err = h.GetExchangePsCurrentMerchant(paymentChannelCostMerchant.PsFixedFeeCurrency, paymentChannelCostMerchant.PsFixedFee)
	if err != nil {
		return err
	}
	if err = h.addEntry(merchantPsFixedFee); err != nil {
		return err
	}

	// 22. realMerchantPsFixedFee
	realMerchantPsFixedFee := h.newEntry(pkg.AccountingEntryTypeRealMerchantPsFixedFee)
	realMerchantPsFixedFee.Amount, err = h.GetExchangePsCurrentCommon(paymentChannelCostMerchant.PsFixedFeeCurrency, paymentChannelCostMerchant.PsFixedFee)
	if err != nil {
		return err
	}
	if err = h.addEntry(realMerchantPsFixedFee); err != nil {
		return err
	}

	// 23. markupMerchantPsFixedFee
	// calculated in order_view

	// 24. psMethodProfit
	// calculated in order_view

	// 25. merchantNetRevenue
	// calculated in order_view

	// 26. psProfitTotal
	// calculated in order_view

	return nil
}

func (h *accountingEntry) processRefundEvent() error {
	var (
		err error
	)

	aes, err := h.accountingRepository.GetByObjectSource(
		h.ctx,
		pkg.ObjectTypeBalanceTransaction,
		h.refund.CreatedOrderId,
		repository.CollectionRefund,
	)

	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	if aes != nil {
		zap.L().Error(
			accountingEntryAlreadyCreated.Message,
			zap.Error(err),
			zap.String("source.type", repository.CollectionRefund),
			zap.String("source.id", h.refund.CreatedOrderId),
		)
		return accountingEntryAlreadyCreated
		// todo: is there must be an update of existing entry, instead of error?
	}

	// info: reversal rates are applied after the transaction has been physically processed by the payment method
	// but refund is the return of payment _before_ of the transaction was physically processed by the payment method.
	// Now, at this moment we can't determine that it is a refund or reversal
	// But we will be able to determine it after getting a settlement from Cardpay
	reason := "reversal"
	if h.refund.IsChargeback {
		reason = "chargeback"
	}
	moneyBackCostMerchant, err := h.getMoneyBackCostMerchant(reason)
	if err != nil {
		return err
	}

	moneyBackCostSystem, err := h.getMoneyBackCostSystem(reason)
	if err != nil {
		return err
	}

	partialRefundCorrection := h.refund.Amount / h.order.ChargeAmount
	if partialRefundCorrection > 1 {
		return accountingEntryErrorRefundExceedsOrderAmount
	}
	// todo: check for past partial refunds for a given order?

	// 1. realRefund
	realRefund := h.newEntry(pkg.AccountingEntryTypeRealRefund)
	realRefund.Amount, err = h.GetExchangePsCurrentCommon(h.refund.Currency, h.refund.Amount)
	if err != nil {
		return err
	}
	realRefund.OriginalAmount = h.refund.Amount
	realRefund.OriginalCurrency = h.refund.Currency
	if err = h.addEntry(realRefund); err != nil {
		return err
	}

	// 2. realRefundTaxFee
	realTaxFee, err := h.accountingRepository.ApplyObjectSource(
		h.ctx,
		pkg.ObjectTypeBalanceTransaction,
		pkg.AccountingEntryTypeRealTaxFee,
		h.order.Id,
		repository.CollectionOrder,
		h.newEntry(""),
	)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return accountingEntryOriginalTaxNotFound
		}

		return err
	}
	realRefundTaxFee := h.newEntry(pkg.AccountingEntryTypeRealRefundTaxFee)
	realRefundTaxFee.Amount = realTaxFee.Amount * partialRefundCorrection
	realRefundTaxFee.Currency = realTaxFee.Currency
	realRefundTaxFee.OriginalAmount = realTaxFee.OriginalAmount * partialRefundCorrection
	realRefundTaxFee.OriginalCurrency = realTaxFee.OriginalCurrency

	// fills with original values, if not deduction, to subtract the same vat amount that was added on payment
	// otherwise local values will be automatically re-calculated with exchange rates for current vat period
	if !h.refundOrder.IsVatDeduction {
		realRefundTaxFee.LocalAmount = realTaxFee.LocalAmount * partialRefundCorrection
		realRefundTaxFee.LocalCurrency = realTaxFee.LocalCurrency
	}

	if err = h.addEntry(realRefundTaxFee); err != nil {
		return err
	}

	// 3. realRefundFee
	realRefundFee := h.newEntry(pkg.AccountingEntryTypeRealRefundFee)
	realRefundFee.Amount = realRefund.Amount * moneyBackCostSystem.Percent
	if err = h.addEntry(realRefundFee); err != nil {
		return err
	}

	// 4. realRefundFixedFee
	realRefundFixedFee := h.newEntry(pkg.AccountingEntryTypeRealRefundFixedFee)
	realRefundFixedFee.Amount, err = h.GetExchangePsCurrentCommon(moneyBackCostSystem.FixAmountCurrency, moneyBackCostSystem.FixAmount)
	if err != nil {
		return err
	}

	if err = h.addEntry(realRefundFixedFee); err != nil {
		return err
	}

	// 5. merchantRefund
	merchantRefund := h.newEntry(pkg.AccountingEntryTypeMerchantRefund)
	merchantRefund.Amount, err = h.GetExchangePsCurrentMerchant(h.refund.Currency, h.refund.Amount)
	if err != nil {
		return err
	}
	if err = h.addEntry(merchantRefund); err != nil {
		return err
	}

	// 6. psMerchantRefundFx
	// calculated in order_view

	// 7. merchantRefundFee
	merchantRefundFee := h.newEntry(pkg.AccountingEntryTypeMerchantRefundFee)
	if moneyBackCostMerchant.IsPaidByMerchant {
		merchantRefundFee.Amount = merchantRefund.Amount * moneyBackCostMerchant.Percent
	}
	if err = h.addEntry(merchantRefundFee); err != nil {
		return err
	}

	// 8. psMarkupMerchantRefundFee
	// calculated in order_view

	merchantRefundFixedFeeCostValue := h.newEntry(pkg.AccountingEntryTypeMerchantRefundFixedFeeCostValue)
	merchantRefundFixedFee := h.newEntry(pkg.AccountingEntryTypeMerchantRefundFixedFee)

	if moneyBackCostMerchant.IsPaidByMerchant {

		// 9. merchantRefundFixedFeeCostValue
		merchantRefundFixedFeeCostValue.Amount, err = h.GetExchangePsCurrentCommon(moneyBackCostMerchant.FixAmountCurrency, moneyBackCostMerchant.FixAmount)
		if err != nil {
			return err
		}

		// 10. merchantRefundFixedFee
		merchantRefundFixedFee.Amount, err = h.GetExchangePsCurrentMerchant(moneyBackCostMerchant.FixAmountCurrency, moneyBackCostMerchant.FixAmount)
		if err != nil {
			return err
		}

		// 11. psMerchantRefundFixedFeeFx
		// calculated in order_view
	}

	if err = h.addEntry(merchantRefundFixedFeeCostValue); err != nil {
		return err
	}
	if err = h.addEntry(merchantRefundFixedFee); err != nil {
		return err
	}

	// 12. psMerchantRefundFixedFeeProfit
	// calculated in order_view

	// 13. reverseTaxFee
	reverseTaxFee := h.newEntry(pkg.AccountingEntryTypeReverseTaxFee)
	merchantTaxFeeCentralBankFx := h.newEntry("")
	if h.country.VatEnabled {
		merchantTaxFeeCostValue, err := h.accountingRepository.ApplyObjectSource(
			h.ctx,
			pkg.ObjectTypeBalanceTransaction,
			pkg.AccountingEntryTypeMerchantTaxFeeCostValue,
			h.order.Id,
			repository.CollectionOrder,
			h.newEntry(""),
		)
		if err != nil {
			return err
		}

		merchantTaxFeeCentralBankFx, err = h.accountingRepository.ApplyObjectSource(
			h.ctx,
			pkg.ObjectTypeBalanceTransaction,
			pkg.AccountingEntryTypeMerchantTaxFeeCentralBankFx,
			h.order.Id,
			repository.CollectionOrder,
			merchantTaxFeeCentralBankFx,
		)
		if err != nil {
			return err
		}

		reverseTaxFee.Amount = (merchantTaxFeeCostValue.Amount + merchantTaxFeeCentralBankFx.Amount) * partialRefundCorrection
		reverseTaxFee.OriginalAmount = (merchantTaxFeeCostValue.OriginalAmount + merchantTaxFeeCentralBankFx.Amount) * partialRefundCorrection
		reverseTaxFee.OriginalCurrency = merchantTaxFeeCostValue.OriginalCurrency
		reverseTaxFee.LocalAmount = (merchantTaxFeeCostValue.LocalAmount + merchantTaxFeeCentralBankFx.Amount) * partialRefundCorrection
		reverseTaxFee.LocalCurrency = merchantTaxFeeCostValue.LocalCurrency
	}
	if err = h.addEntry(reverseTaxFee); err != nil {
		return err
	}

	// 14. reverseTaxFeeDelta
	// 15. psReverseTaxFeeDelta
	reverseTaxFeeDelta := h.newEntry(pkg.AccountingEntryTypeReverseTaxFeeDelta)
	psReverseTaxFeeDelta := h.newEntry(pkg.AccountingEntryTypePsReverseTaxFeeDelta)

	// do not calculate reverseTaxFeeDelta and psReverseTaxFeeDelta
	// if merchantTaxFeeCentralBankFx amount is zero.
	// @see https://protocolone.tpondemand.com/entity/196061
	if h.country.VatEnabled && merchantTaxFeeCentralBankFx.Amount > 0 {
		// #192161 calculation rules changed:
		// first, restoring tax amount from merchantRefund,
		// then converting restored tax amount from merchant currency to vat currency by centralbank rate,
		// after that converting it back from vat currency  to merchant currency by stock rate,
		// next getting Centralbank fx for restored value as difference between converted and restored values,
		// and finally getting difference between old merchantTaxFeeCentralBankFx amount and calculated new.
		amountVatRestored := tools.GetPercentPartFromAmount(merchantRefund.Amount, h.order.Tax.Rate)
		amountVatCb, err := h.GetExchangeCbCurrentCommon(h.order.GetMerchantRoyaltyCurrency(), amountVatRestored)
		if err != nil {
			return err
		}
		amountMerchantStock, err := h.GetExchangeStockCurrentCommon(h.country.GetVatCurrencyCode(), amountVatCb)
		if err != nil {
			return err
		}
		amountFxRestored := amountMerchantStock - amountVatRestored
		amountResult := merchantTaxFeeCentralBankFx.Amount - amountFxRestored

		if amountResult < 0 {
			psReverseTaxFeeDelta.Amount = -1 * amountResult
		} else {
			reverseTaxFeeDelta.Amount = amountResult
		}
	}

	if err = h.addEntry(reverseTaxFeeDelta); err != nil {
		return err
	}
	if err = h.addEntry(psReverseTaxFeeDelta); err != nil {
		return err
	}

	// 16. merchantReverseTaxFee
	// calculated in order_view

	// 17. merchantReverseRevenue
	// calculated in order_view

	// 18. psRefundProfit
	// calculated in order_view

	return nil
}

func (h *accountingEntry) GetExchangePsCurrentCommon(from string, amount float64) (float64, error) {
	to := h.order.GetMerchantRoyaltyCurrency()

	if to == from {
		return amount, nil
	}
	return h.GetExchangeCurrentCommon(&currenciespb.ExchangeCurrencyCurrentCommonRequest{
		From:              from,
		To:                to,
		RateType:          currenciespb.RateTypePaysuper,
		ExchangeDirection: currenciespb.ExchangeDirectionBuy,
		Amount:            amount,
	})
}

func (h *accountingEntry) GetExchangeStockCurrentCommon(from string, amount float64) (float64, error) {
	to := h.order.GetMerchantRoyaltyCurrency()

	if to == from {
		return amount, nil
	}
	return h.GetExchangeCurrentCommon(&currenciespb.ExchangeCurrencyCurrentCommonRequest{
		From:              from,
		To:                to,
		RateType:          currenciespb.RateTypeStock,
		ExchangeDirection: currenciespb.ExchangeDirectionSell,
		Amount:            amount,
	})
}

func (h *accountingEntry) GetExchangePsCurrentMerchant(from string, amount float64) (float64, error) {
	to := h.order.GetMerchantRoyaltyCurrency()

	if to == from {
		return amount, nil
	}

	return h.GetExchangeCurrentMerchant(&currenciespb.ExchangeCurrencyCurrentForMerchantRequest{
		From:     from,
		To:       to,
		RateType: currenciespb.RateTypePaysuper,
		// use exchange direction SELL for merchant exchages
		// @see https://protocolone.tpondemand.com/entity/196061
		ExchangeDirection: currenciespb.ExchangeDirectionSell,
		MerchantId:        h.order.GetMerchantId(),
		Amount:            amount,
	})
}

func (h *accountingEntry) GetExchangeCbCurrentCommon(from string, amount float64) (float64, error) {
	to := h.country.GetVatCurrencyCode()

	if to == from {
		return amount, nil
	}

	return h.GetExchangeCurrentCommon(&currenciespb.ExchangeCurrencyCurrentCommonRequest{
		From:              from,
		To:                to,
		RateType:          currenciespb.RateTypeCentralbanks,
		ExchangeDirection: currenciespb.ExchangeDirectionSell,
		Source:            h.country.VatCurrencyRatesSource,
		Amount:            amount,
	})
}

func (h *accountingEntry) GetExchangeCurrentMerchant(req *currenciespb.ExchangeCurrencyCurrentForMerchantRequest) (float64, error) {

	if req.Amount == 0 || req.From == req.To {
		return req.Amount, nil
	}

	rsp, err := h.curService.ExchangeCurrencyCurrentForMerchant(h.ctx, req)

	if err != nil {
		zap.L().Error(
			pkg.ErrorGrpcServiceCallFailed,
			zap.Error(err),
			zap.String(errorFieldService, "CurrencyRatesService"),
			zap.String(errorFieldMethod, "ExchangeCurrencyCurrentForMerchantRequest"),
			zap.Any(errorFieldRequest, req),
			zap.Any(errorFieldEntrySource, h.order.Id),
		)

		return 0, accountingEntryErrorExchangeFailed
	}

	return rsp.ExchangedAmount, nil
}

func (h *accountingEntry) GetExchangeCurrentCommon(req *currenciespb.ExchangeCurrencyCurrentCommonRequest) (float64, error) {

	if req.Amount == 0 || req.From == req.To {
		return req.Amount, nil
	}

	rsp, err := h.curService.ExchangeCurrencyCurrentCommon(h.ctx, req)

	if err != nil {
		zap.L().Error(
			pkg.ErrorGrpcServiceCallFailed,
			zap.Error(err),
			zap.String(errorFieldService, "CurrencyRatesService"),
			zap.String(errorFieldMethod, "ExchangeCurrencyCurrentCommon"),
			zap.Any(errorFieldRequest, req),
			zap.Any(errorFieldEntrySource, h.order.Id),
		)

		return 0, accountingEntryErrorExchangeFailed
	}

	return rsp.ExchangedAmount, nil
}

func (h *accountingEntry) addEntry(entry *billingpb.AccountingEntry) error {
	if _, ok := availableAccountingEntries[entry.Type]; !ok {
		return accountingEntryErrorUnknownEntry
	}

	if _, ok := availableAccountingEntriesSourceTypes[entry.Source.Type]; !ok {
		return accountingEntryErrorUnknownSourceType
	}

	if entry.Source.Id == "" {
		return accountingEntryErrorInvalidSourceId
	}

	if entry.OriginalAmount == 0 && entry.OriginalCurrency == "" {
		entry.OriginalAmount = entry.Amount
		entry.OriginalCurrency = entry.Currency
	}
	if entry.LocalAmount == 0 && entry.LocalCurrency == "" && entry.Country != "" {
		var rateType string
		var rateSource string
		if h.country.VatEnabled {
			// Use VatCurrency as local currency, instead of country currency.
			// It because of some countries of EU,
			// that use national currencies but pays vat in euro
			if h.country.VatCurrency == "" {
				return accountingEntryVatCurrencyNotSet
			}
			entry.LocalCurrency = h.country.VatCurrency
			rateType = currenciespb.RateTypeCentralbanks
			rateSource = h.country.VatCurrencyRatesSource
		} else {
			priceGroup, err := h.Service.priceGroupRepository.GetById(h.ctx, h.country.PriceGroupId)
			if err != nil {
				return err
			}
			entry.LocalCurrency = priceGroup.Currency
			rateType = currenciespb.RateTypeOxr
			rateSource = ""
		}

		if entry.LocalCurrency == entry.OriginalCurrency {
			entry.LocalAmount = entry.OriginalAmount
		} else {
			req := &currenciespb.ExchangeCurrencyCurrentCommonRequest{
				From:              entry.OriginalCurrency,
				To:                entry.LocalCurrency,
				RateType:          rateType,
				ExchangeDirection: currenciespb.ExchangeDirectionBuy,
				Source:            rateSource,
				Amount:            entry.OriginalAmount,
			}

			if req.Amount != 0 && req.From != req.To {

				rsp, err := h.curService.ExchangeCurrencyCurrentCommon(h.ctx, req)

				if err != nil {
					zap.L().Error(
						pkg.ErrorGrpcServiceCallFailed,
						zap.Error(err),
						zap.String(errorFieldService, "CurrencyRatesService"),
						zap.String(errorFieldMethod, "ExchangeCurrencyCurrentCommon"),
						zap.String(errorFieldEntry, entry.Type),
						zap.Any(errorFieldRequest, req),
						zap.Any(errorFieldEntrySource, entry.Source),
					)

					return accountingEntryErrorExchangeFailed
				} else {
					entry.LocalAmount = rsp.ExchangedAmount
				}
			}
		}
	}

	entry.Amount = tools.ToPrecise(entry.Amount)
	entry.OriginalAmount = tools.ToPrecise(entry.OriginalAmount)
	entry.LocalAmount = tools.ToPrecise(entry.LocalAmount)

	h.accountingEntries = append(h.accountingEntries, entry)

	return nil
}

func (h *accountingEntry) saveAccountingEntries(
	owr repository.OrderViewRepositoryInterface,
	plr repository.PaylinkRepositoryInterface,
	plvr repository.PaylinkVisitRepositoryInterface,
) error {
	err := h.accountingRepository.MultipleInsert(h.ctx, h.accountingEntries)

	if err != nil {
		return err
	}

	var ids []string
	var paylinks = map[string]string{}
	if h.order != nil {
		ids = append(ids, h.order.Id)
		if h.order.Issuer.ReferenceType == pkg.OrderIssuerReferenceTypePaylink && h.order.Issuer.Reference != "" {
			paylinks[h.order.Issuer.Reference] = h.order.Project.MerchantId
		}
	}

	if h.refund != nil && h.refundOrder != nil {
		ids = append(ids, h.refundOrder.Id)
		if h.refundOrder.Issuer.ReferenceType == pkg.OrderIssuerReferenceTypePaylink && h.refundOrder.Issuer.Reference != "" {
			paylinks[h.order.Issuer.Reference] = h.refundOrder.Project.MerchantId
		}
	}

	if len(ids) == 0 {
		return nil
	}

	err = h.Service.updateOrderView(h.ctx, ids)
	if err != nil {
		return err
	}

	for paylinkId, merchantId := range paylinks {
		pl, err := plr.GetByIdAndMerchant(h.ctx, paylinkId, merchantId)
		if err != nil {
			return err
		}

		visits, err := plvr.CountPaylinkVisits(h.ctx, paylinkId, 0, 0)
		if err == nil {
			pl.Visits = int32(visits)
		}

		stat, err := owr.GetPaylinkStat(h.ctx, paylinkId, merchantId, 0, 0)
		if err != nil {
			return err
		}

		pl.TotalTransactions = stat.TotalTransactions
		pl.ReturnsCount = stat.ReturnsCount
		pl.SalesCount = stat.SalesCount
		pl.TransactionsCurrency = stat.TransactionsCurrency
		pl.GrossTotalAmount = stat.GrossTotalAmount
		pl.GrossSalesAmount = stat.GrossSalesAmount
		pl.GrossReturnsAmount = stat.GrossReturnsAmount
		pl.IsExpired = pl.IsPaylinkExpired()
		pl.UpdateConversion()

		err = h.Service.paylinkRepository.UpdateTotalStat(h.ctx, pl)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *accountingEntry) newEntry(entryType string) *billingpb.AccountingEntry {

	var (
		createdTime        = ptypes.TimestampNow()
		source             *billingpb.AccountingEntrySource
		merchantId         = ""
		currency           = ""
		country            = ""
		operatingCompanyId = ""
	)
	if h.refund != nil {
		if h.refundOrder != nil {
			createdTime = h.refundOrder.PaymentMethodOrderClosedAt
			merchantId = h.refundOrder.GetMerchantId()
			currency = h.refundOrder.GetMerchantRoyaltyCurrency()
			operatingCompanyId = h.refundOrder.OperatingCompanyId
		}
		source = &billingpb.AccountingEntrySource{
			Id:   h.refund.CreatedOrderId,
			Type: repository.CollectionRefund,
		}
	} else {
		if h.order != nil {
			createdTime = h.order.PaymentMethodOrderClosedAt
			source = &billingpb.AccountingEntrySource{
				Id:   h.order.Id,
				Type: repository.CollectionOrder,
			}
			merchantId = h.order.GetMerchantId()
			currency = h.order.GetMerchantRoyaltyCurrency()
			operatingCompanyId = h.order.OperatingCompanyId
		} else {
			if h.merchant != nil {
				createdTime = ptypes.TimestampNow()
				source = &billingpb.AccountingEntrySource{
					Id:   h.merchant.Id,
					Type: repository.CollectionMerchant,
				}
				merchantId = h.merchant.Id
				currency = h.merchant.GetPayoutCurrency()
				operatingCompanyId = h.merchant.OperatingCompanyId
			}
		}
	}

	if h.country != nil {
		country = h.country.IsoCodeA2
	}

	return &billingpb.AccountingEntry{
		Id:                 primitive.NewObjectID().Hex(),
		Object:             pkg.ObjectTypeBalanceTransaction,
		Type:               entryType,
		Source:             source,
		MerchantId:         merchantId,
		Status:             pkg.BalanceTransactionStatusAvailable,
		CreatedAt:          createdTime,
		Country:            country,
		Currency:           currency,
		OperatingCompanyId: operatingCompanyId,
	}
}

func (h *accountingEntry) getPaymentChannelCostSystem() (*billingpb.PaymentChannelCostSystem, error) {
	name, err := h.order.GetCostPaymentMethodName()
	if err != nil {
		return nil, err
	}

	cost, err := h.Service.paymentChannelCostSystemRepository.Find(h.ctx, name, h.country.PayerTariffRegion, h.country.IsoCodeA2, h.getMccCode(), h.getOperatingCompanyId())

	if err != nil {
		zap.L().Error(
			accountingEntryErrorSystemCommissionNotFound.Message,
			zap.Error(err),
			zap.String("payment_method", name),
			zap.String("region", h.country.PayerTariffRegion),
			zap.String("country", h.country.IsoCodeA2),
		)

		return nil, accountingEntryErrorSystemCommissionNotFound
	}
	return cost, nil
}

func (h *accountingEntry) getPaymentChannelCostMerchant(amount float64) (*billingpb.PaymentChannelCostMerchant, error) {
	name, err := h.order.GetCostPaymentMethodName()
	if err != nil {
		return nil, err
	}

	req := &billingpb.PaymentChannelCostMerchantRequest{
		MerchantId:     h.order.GetMerchantId(),
		Name:           name,
		PayoutCurrency: h.order.GetMerchantRoyaltyCurrency(),
		Amount:         amount,
		Region:         h.country.PayerTariffRegion,
		Country:        h.country.IsoCodeA2,
		MccCode:        h.getMccCode(),
	}
	cost, err := h.Service.getPaymentChannelCostMerchant(h.ctx, req)

	if err != nil {
		zap.L().Error(
			accountingEntryErrorMerchantCommissionNotFound.Message,
			zap.Error(err),
			zap.String("project", h.order.GetProjectId()),
			zap.String("payment_method", h.order.GetPaymentMethodId()),
		)

		return nil, accountingEntryErrorMerchantCommissionNotFound
	}
	return cost, nil
}

func (h *accountingEntry) getMoneyBackCostMerchant(reason string) (*billingpb.MoneyBackCostMerchant, error) {
	name, err := h.order.GetCostPaymentMethodName()

	if err != nil {
		return nil, err
	}

	paymentAt, _ := ptypes.Timestamp(h.order.PaymentMethodOrderClosedAt)
	refundAt, _ := ptypes.Timestamp(h.refund.CreatedAt)

	data := &billingpb.MoneyBackCostMerchantRequest{
		MerchantId:     h.order.GetMerchantId(),
		Name:           name,
		PayoutCurrency: h.order.GetMerchantRoyaltyCurrency(),
		UndoReason:     reason,
		Region:         h.country.PayerTariffRegion,
		Country:        h.country.IsoCodeA2,
		PaymentStage:   1,
		Days:           int32(refundAt.Sub(paymentAt).Hours() / 24),
		MccCode:        h.getMccCode(),
	}
	return h.Service.getMoneyBackCostMerchant(h.ctx, data)
}

func (h *accountingEntry) getMoneyBackCostSystem(reason string) (*billingpb.MoneyBackCostSystem, error) {
	name, err := h.order.GetCostPaymentMethodName()

	if err != nil {
		return nil, err
	}

	paymentAt, _ := ptypes.Timestamp(h.order.PaymentMethodOrderClosedAt)
	refundAt, _ := ptypes.Timestamp(h.refund.CreatedAt)

	data := &billingpb.MoneyBackCostSystemRequest{
		Name:               name,
		PayoutCurrency:     h.order.GetMerchantRoyaltyCurrency(),
		Region:             h.country.PayerTariffRegion,
		Country:            h.country.IsoCodeA2,
		PaymentStage:       1,
		Days:               int32(refundAt.Sub(paymentAt).Hours() / 24),
		UndoReason:         reason,
		MccCode:            h.getMccCode(),
		OperatingCompanyId: h.getOperatingCompanyId(),
	}
	return h.Service.getMoneyBackCostSystem(h.ctx, data)
}

func (h *accountingEntry) getMccCode() string {
	if h.refundOrder != nil && h.refundOrder.MccCode != "" {
		return h.refundOrder.MccCode
	}
	if h.order != nil && h.order.MccCode != "" {
		return h.order.MccCode
	}
	if h.merchant != nil && h.merchant.MccCode != "" {
		return h.merchant.MccCode
	}
	return ""
}

func (h *accountingEntry) getOperatingCompanyId() string {
	if h.refundOrder != nil && h.refundOrder.OperatingCompanyId != "" {
		return h.refundOrder.OperatingCompanyId
	}
	if h.order != nil && h.order.OperatingCompanyId != "" {
		return h.order.OperatingCompanyId
	}
	if h.merchant != nil && h.merchant.OperatingCompanyId != "" {
		return h.merchant.OperatingCompanyId
	}
	return ""
}

func (s *Service) FixTaxes(ctx context.Context) error {
	taxAccountingEntriesList := []string{
		pkg.AccountingEntryTypeRealTaxFee,
		pkg.AccountingEntryTypeRealRefundTaxFee,
	}
	aes, err := s.accountingRepository.GetByTypeWithTaxes(ctx, taxAccountingEntriesList)

	if err != nil {
		return err
	}

	count := len(aes)

	if count == 0 {
		return nil
	}

	zap.S().Info("found " + strconv.FormatInt(int64(count), 10) + " entries")

	hasErrors := false

	var list []*billingpb.AccountingEntry

	for _, ae := range aes {
		order, err := s.getOrderById(ctx, ae.Source.Id)

		if err != nil {
			zap.L().Error(
				"get order error",
				zap.Error(err),
			)
			hasErrors = true
			continue
		}

		orderTaxAmount := order.GetTaxAmountInChargeCurrency()
		req := &currenciespb.ExchangeCurrencyByDateCommonRequest{
			From:              order.ChargeCurrency,
			To:                ae.Currency,
			RateType:          currenciespb.RateTypePaysuper,
			ExchangeDirection: currenciespb.ExchangeDirectionBuy,
			Source:            "",
			Amount:            orderTaxAmount,
			Datetime:          order.PaymentMethodOrderClosedAt,
		}

		ae.Amount, err = s.exchangeCurrencyByDateCommon(ctx, req)
		if err != nil {
			zap.L().Error(
				"exchangeCurrencyByDateCommon failed for amount",
				zap.String("orderId", order.Id),
				zap.Error(err),
			)
			hasErrors = true
			continue
		}
		ae.OriginalAmount = orderTaxAmount
		ae.OriginalCurrency = order.ChargeCurrency

		country, err := s.country.GetByIsoCodeA2(ctx, ae.Country)
		if err != nil {
			zap.L().Error(
				"tax fix get country failed",
				zap.String("orderId", order.Id),
				zap.Error(err),
			)
			hasErrors = true
			continue
		}

		var rateType string
		var rateSource string

		if country.VatEnabled {
			rateType = currenciespb.RateTypeCentralbanks
			rateSource = country.VatCurrencyRatesSource
		} else {
			rateType = currenciespb.RateTypeOxr
			rateSource = ""
		}

		if ae.LocalCurrency == ae.OriginalCurrency {
			ae.LocalAmount = ae.OriginalAmount
		} else {

			req := &currenciespb.ExchangeCurrencyByDateCommonRequest{
				From:              ae.OriginalCurrency,
				To:                ae.LocalCurrency,
				RateType:          rateType,
				ExchangeDirection: currenciespb.ExchangeDirectionBuy,
				Source:            rateSource,
				Amount:            ae.OriginalAmount,
				Datetime:          order.PaymentMethodOrderClosedAt,
			}

			la, err := s.exchangeCurrencyByDateCommon(ctx, req)
			if err != nil {
				zap.L().Error(
					"exchangeCurrencyByDateCommon failed for local amount",
					zap.String("orderId", order.Id),
					zap.Error(err),
				)
				hasErrors = true
				continue
			} else {
				ae.LocalAmount = la
			}
		}

		list = append(list, ae)
	}

	if len(list) == 0 {
		return nil
	}

	if err = s.accountingRepository.BulkWrite(ctx, aes); err != nil {
		return err
	}

	if hasErrors {
		return errors.New("errors occurred while processing tax fixes")
	}

	return nil

}

func (s *Service) exchangeCurrencyByDateCommon(ctx context.Context, req *currenciespb.ExchangeCurrencyByDateCommonRequest) (float64, error) {
	if req.Amount == 0 || req.From == req.To {
		return req.Amount, nil
	}

	rsp, err := s.curService.ExchangeCurrencyByDateCommon(ctx, req)

	if err != nil {
		zap.L().Error(
			pkg.ErrorGrpcServiceCallFailed,
			zap.Error(err),
			zap.String(errorFieldService, "CurrencyRatesService"),
			zap.String(errorFieldMethod, "ExchangeCurrencyByDateCommon"),
			zap.String("request date", ptypes.TimestampString(req.Datetime)),
			zap.Any(errorFieldRequest, req),
		)

		return 0, accountingEntryErrorExchangeFailed
	}

	return rsp.ExchangedAmount, nil
}
