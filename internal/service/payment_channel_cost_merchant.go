package service

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/currenciespb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"sort"
)

var (
	errorPaymentChannelMerchantGetAll           = errors.NewBillingServerErrorMsg("pcm000001", "can't get list of payment channel setting for merchant")
	errorPaymentChannelMerchantGet              = errors.NewBillingServerErrorMsg("pcm000002", "can't get payment channel setting for merchant")
	errorPaymentChannelMerchantSetFailed        = errors.NewBillingServerErrorMsg("pcm000003", "can't set payment channel setting for merchant")
	errorPaymentChannelMerchantDelete           = errors.NewBillingServerErrorMsg("pcm000004", "can't delete payment channel setting for merchant")
	errorPaymentChannelMerchantCurrency         = errors.NewBillingServerErrorMsg("pcm000005", "currency not supported")
	errorPaymentChannelMerchantCostAlreadyExist = errors.NewBillingServerErrorMsg("pcm000006", "cost with specified parameters already exist")
	errorCostMatchedToAmountNotFound            = errors.NewBillingServerErrorMsg("pcm000007", "cost matched to amount not found")
	errorPaymentChannelMccCode                  = errors.NewBillingServerErrorMsg("pcm000008", "mcc code not supported")
	errorCostMatchedNotFound                    = errors.NewBillingServerErrorMsg("pcm000009", "cost matched not found")
	errorMerchantTariffUpdate                   = errors.NewBillingServerErrorMsg("pcm000010", "can't update merchant tariffs")
	errorPaymentChannelCostIdNotFound           = errors.NewBillingServerErrorMsg("pcm000011", "payment cost identifier not found")
)

func (s *Service) GetAllPaymentChannelCostMerchant(
	ctx context.Context,
	req *billingpb.PaymentChannelCostMerchantListRequest,
	res *billingpb.PaymentChannelCostMerchantListResponse,
) error {
	val, err := s.paymentChannelCostMerchantRepository.GetAllForMerchant(ctx, req.MerchantId)
	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorPaymentChannelMerchantGetAll
		return nil
	}

	res.Status = billingpb.ResponseStatusOk
	res.Item = &billingpb.PaymentChannelCostMerchantList{Items: val}

	return nil
}

func (s *Service) GetPaymentChannelCostMerchant(
	ctx context.Context,
	req *billingpb.PaymentChannelCostMerchantRequest,
	res *billingpb.PaymentChannelCostMerchantResponse,
) error {
	val, err := s.getPaymentChannelCostMerchant(ctx, req)
	if err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = errorPaymentChannelMerchantGet
		return nil
	}

	res.Status = billingpb.ResponseStatusOk
	res.Item = val

	return nil
}

func (s *Service) SetPaymentChannelCostMerchant(
	ctx context.Context,
	req *billingpb.PaymentChannelCostMerchant,
	res *billingpb.PaymentChannelCostMerchantResponse,
) error {
	err := s.validatePaymentChannelCostMerchant(ctx, req, res)

	req.IsActive = true

	if req.Id != "" {
		val, err := s.paymentChannelCostMerchantRepository.GetById(ctx, req.Id)
		if err != nil {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = errorPaymentChannelMerchantSetFailed
			return nil
		}
		req.Id = val.Id
		req.MerchantId = val.MerchantId
		req.CreatedAt = val.CreatedAt
		err = s.paymentChannelCostMerchantRepository.Update(ctx, req)
	} else {
		req.Id = primitive.NewObjectID().Hex()
		err = s.paymentChannelCostMerchantRepository.Insert(ctx, req)
	}
	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorPaymentChannelMerchantSetFailed

		if mongodb.IsDuplicate(err) {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = errorPaymentChannelMerchantCostAlreadyExist
		}

		return nil
	}

	if err := s.merchantRepository.UpdateTariffs(ctx, req.MerchantId, req); err != nil {
		zap.L().Error(pkg.MethodFinishedWithError, zap.Error(err))
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorMerchantTariffUpdate
		return nil
	}

	res.Status = billingpb.ResponseStatusOk
	res.Item = req

	return nil
}

func (s *Service) SetAllPaymentChannelCostMerchant(
	ctx context.Context,
	req *billingpb.SetAllPaymentChannelCostMerchantRequest,
	res *billingpb.SetAllPaymentChannelCostMerchantResponse,
) error {
	for _, cost := range req.Costs {
		if cost.Id == "" {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = errorPaymentChannelCostIdNotFound

			return nil
		}

		cost.MerchantId = req.MerchantId
		validateRes := &billingpb.PaymentChannelCostMerchantResponse{}
		_ = s.validatePaymentChannelCostMerchant(ctx, cost, validateRes)

		if validateRes.Status != billingpb.ResponseStatusOk {
			res.Status = validateRes.Status
			res.Message = validateRes.Message

			return nil
		}
	}

	for _, cost := range req.Costs {
		cost.MerchantId = req.MerchantId
		err := s.paymentChannelCostMerchantRepository.Update(ctx, cost)

		if err != nil {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = errorPaymentChannelMerchantSetFailed

			if mongodb.IsDuplicate(err) {
				res.Status = billingpb.ResponseStatusBadData
				res.Message = errorPaymentChannelMerchantCostAlreadyExist
			}

			return nil
		}

		if err := s.merchantRepository.UpdateTariffs(ctx, req.MerchantId, cost); err != nil {
			zap.L().Error(pkg.MethodFinishedWithError, zap.Error(err))
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = errorMerchantTariffUpdate
			return nil
		}

		res.Items = append(res.Items, cost)
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) validatePaymentChannelCostMerchant(
	ctx context.Context,
	req *billingpb.PaymentChannelCostMerchant,
	res *billingpb.PaymentChannelCostMerchantResponse,
) error {
	var err error

	if _, err = s.merchantRepository.GetById(ctx, req.MerchantId); err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = merchantErrorNotFound
		return nil
	}

	if req.Country != "" {
		country, err := s.country.GetByIsoCodeA2(ctx, req.Country)
		if err != nil {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = errorCountryNotFound
			return nil
		}
		req.Region = country.PayerTariffRegion
	} else {
		exists := s.country.IsTariffRegionSupported(req.Region)
		if !exists {
			res.Status = billingpb.ResponseStatusNotFound
			res.Message = errorCountryRegionNotExists
			return nil
		}
	}

	sCurr, err := s.curService.GetSettlementCurrencies(ctx, &currenciespb.EmptyRequest{})
	if err != nil {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPaymentChannelMerchantCurrency
		return nil
	}
	if !helper.Contains(sCurr.Currencies, req.PayoutCurrency) {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPaymentChannelMerchantCurrency
		return nil
	}
	if !helper.Contains(sCurr.Currencies, req.PsFixedFeeCurrency) {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPaymentChannelMerchantCurrency
		return nil
	}
	if !helper.Contains(sCurr.Currencies, req.MethodFixAmountCurrency) {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPaymentChannelMerchantCurrency
		return nil
	}
	if !helper.Contains(pkg.SupportedMccCodes, req.MccCode) {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errorPaymentChannelMccCode
		return nil
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) DeletePaymentChannelCostMerchant(
	ctx context.Context,
	req *billingpb.PaymentCostDeleteRequest,
	res *billingpb.ResponseError,
) error {
	pc, err := s.paymentChannelCostMerchantRepository.GetById(ctx, req.Id)
	if err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = errorCostRateNotFound
		return nil
	}
	err = s.paymentChannelCostMerchantRepository.Delete(ctx, pc)
	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorPaymentChannelMerchantDelete
		return nil
	}

	res.Status = billingpb.ResponseStatusOk
	return nil
}

func (s *Service) getPaymentChannelCostMerchant(
	ctx context.Context,
	req *billingpb.PaymentChannelCostMerchantRequest,
) (*billingpb.PaymentChannelCostMerchant, error) {
	val, err := s.paymentChannelCostMerchantRepository.Find(
		ctx,
		req.MerchantId,
		req.Name,
		req.PayoutCurrency,
		req.Region,
		req.Country,
		req.MccCode,
	)

	if err != nil {
		return nil, err
	}

	if val == nil {
		return nil, errorCostMatchedNotFound
	}

	var matched []*kvIntFloat
	for _, set := range val {
		for k, i := range set.Set {
			if req.Amount >= i.MinAmount {
				matched = append(matched, &kvIntFloat{k, i.MinAmount})
			}
		}
		if len(matched) == 0 {
			continue
		}

		sort.Slice(matched, func(i, j int) bool {
			return matched[i].Value > matched[j].Value
		})
		return set.Set[matched[0].Key], nil
	}

	return nil, errorCostMatchedToAmountNotFound
}
