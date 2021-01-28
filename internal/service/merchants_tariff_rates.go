package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
)

func (s *Service) getMerchantTariffRates(
	ctx context.Context,
	in *billingpb.GetMerchantTariffRatesRequest,
) (*billingpb.GetMerchantTariffRatesResponseItems, error) {
	mccCode, err := getMccByOperationsType(in.MerchantOperationsType)

	if err != nil {
		return nil, err
	}

	payment, err := s.merchantPaymentTariffsRepository.Find(ctx, in.HomeRegion, in.PayerRegion, mccCode, in.MinAmount, in.MaxAmount)

	if err != nil {
		return nil, merchantErrorUnknown
	}

	if len(payment) <= 0 {
		return nil, merchantTariffsNotFound
	}

	settings, err := s.merchantTariffsSettingsRepository.GetByMccCode(ctx, mccCode)

	if err != nil {
		return nil, merchantTariffsNotFound
	}

	tariffs := &billingpb.GetMerchantTariffRatesResponseItems{
		Payment:       payment,
		Refund:        settings.Refund,
		Chargeback:    settings.Chargeback,
		Payout:        settings.Payout,
		MinimalPayout: settings.MinimalPayout,
		MccCode:       settings.MccCode,
	}

	return tariffs, nil
}

func (s *Service) setMerchantTariffRates(
	ctx context.Context,
	merchant *billingpb.Merchant,
	merchantHomeRegion, merchantOperationsType string,
) (*billingpb.Merchant, error) {
	mccCode, err := getMccByOperationsType(merchantOperationsType)

	if err != nil {
		return nil, err
	}

	query := &billingpb.GetMerchantTariffRatesRequest{
		HomeRegion:             merchantHomeRegion,
		MerchantOperationsType: merchantOperationsType,
	}
	tariffs, err := s.getMerchantTariffRates(ctx, query)

	if err != nil {
		return nil, merchantErrorUnknown
	}

	merchantPayoutCurrency := merchant.GetPayoutCurrency()

	if merchantPayoutCurrency == "" {
		return nil, merchantErrorCurrencyNotSet
	}

	payoutTariff, ok := tariffs.Payout[merchantPayoutCurrency]

	if !ok {
		return nil, merchantErrorNoTariffsInPayoutCurrency
	}

	minimalPayoutLimit, ok := tariffs.MinimalPayout[merchantPayoutCurrency]

	if !ok {
		return nil, merchantErrorNoTariffsInPayoutCurrency
	}

	merchant.Tariff = &billingpb.MerchantTariff{
		Payment:       tariffs.Payment,
		Payout:        payoutTariff,
		HomeRegion:    merchantHomeRegion,
		Chargeback:    tariffs.Chargeback,
		Refund:        tariffs.Refund,
		MinimalPayout: tariffs.MinimalPayout,
	}
	merchant.MccCode = mccCode
	merchant.MinimalPayoutLimit = minimalPayoutLimit
	merchant.MerchantOperationsType = merchantOperationsType

	paymentTariffs, refundTariffs, chargebackTariffs := s.transformMerchantTariffRates(merchant, tariffs)
	refundTariffs = append(refundTariffs, chargebackTariffs...)

	if len(paymentTariffs) > 0 {
		err = s.paymentChannelCostMerchantRepository.DeleteAndInsertMany(ctx, merchant.Id, paymentTariffs)

		if err != nil {
			return nil, merchantErrorUnknown
		}
	}

	if len(refundTariffs) > 0 {
		err = s.moneyBackCostMerchantRepository.DeleteAndInsertMany(ctx, merchant.Id, refundTariffs)

		if err != nil {
			return nil, merchantErrorUnknown
		}
	}

	return merchant, nil
}

func (s *Service) transformMerchantTariffRates(
	merchant *billingpb.Merchant,
	tariffs *billingpb.GetMerchantTariffRatesResponseItems,
) ([]*billingpb.PaymentChannelCostMerchant, []*billingpb.MoneyBackCostMerchant, []*billingpb.MoneyBackCostMerchant) {
	payment := make([]*billingpb.PaymentChannelCostMerchant, 0)
	refund := make([]*billingpb.MoneyBackCostMerchant, 0)
	chargeback := make([]*billingpb.MoneyBackCostMerchant, 0)
	tsNow := ptypes.TimestampNow()

	for _, v := range tariffs.Payment {
		cost := &billingpb.PaymentChannelCostMerchant{
			Id:                      primitive.NewObjectID().Hex(),
			MerchantId:              merchant.Id,
			Name:                    strings.ToUpper(v.MethodName),
			PayoutCurrency:          merchant.GetPayoutCurrency(),
			MinAmount:               v.MinAmount,
			MaxAmount:               v.MaxAmount,
			Region:                  v.PayerRegion,
			MethodPercent:           v.MethodPercentFee,
			MethodFixAmount:         v.MethodFixedFee,
			MethodFixAmountCurrency: v.MethodFixedFeeCurrency,
			PsPercent:               v.PsPercentFee,
			PsFixedFee:              v.PsFixedFee,
			PsFixedFeeCurrency:      v.PsFixedFeeCurrency,
			CreatedAt:               tsNow,
			UpdatedAt:               tsNow,
			IsActive:                true,
			MccCode:                 merchant.MccCode,
		}
		payment = append(payment, cost)
	}

	for _, tariffRegion := range pkg.SupportedTariffRegions {
		for _, v := range tariffs.Refund {
			cost := &billingpb.MoneyBackCostMerchant{
				Id:                primitive.NewObjectID().Hex(),
				MerchantId:        merchant.Id,
				Name:              strings.ToUpper(v.MethodName),
				PayoutCurrency:    merchant.GetPayoutCurrency(),
				UndoReason:        pkg.UndoReasonReversal,
				Region:            tariffRegion,
				Country:           "",
				DaysFrom:          0,
				PaymentStage:      1,
				Percent:           v.MethodPercentFee,
				FixAmount:         v.MethodFixedFee,
				FixAmountCurrency: v.MethodFixedFeeCurrency,
				IsPaidByMerchant:  v.IsPaidByMerchant,
				CreatedAt:         tsNow,
				UpdatedAt:         tsNow,
				IsActive:          true,
				MccCode:           merchant.MccCode,
			}
			refund = append(refund, cost)
		}

		for _, v := range tariffs.Chargeback {
			cost := &billingpb.MoneyBackCostMerchant{
				Id:                primitive.NewObjectID().Hex(),
				MerchantId:        merchant.Id,
				Name:              strings.ToUpper(v.MethodName),
				PayoutCurrency:    merchant.GetPayoutCurrency(),
				UndoReason:        pkg.UndoReasonChargeback,
				Region:            tariffRegion,
				Country:           "",
				DaysFrom:          0,
				PaymentStage:      1,
				Percent:           v.MethodPercentFee,
				FixAmount:         v.MethodFixedFee,
				FixAmountCurrency: v.MethodFixedFeeCurrency,
				IsPaidByMerchant:  v.IsPaidByMerchant,
				CreatedAt:         tsNow,
				UpdatedAt:         tsNow,
				IsActive:          true,
				MccCode:           merchant.MccCode,
			}
			chargeback = append(chargeback, cost)
		}
	}

	return payment, refund, chargeback
}
