package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.uber.org/zap"
)

var (
	errorCountryNotFound        = errors.NewBillingServerErrorMsg("co000001", "country not found")
	errorCountryRegionNotExists = errors.NewBillingServerErrorMsg("co000002", "region not exists")
	errorCountryOrderIdRequired = errors.NewBillingServerErrorMsg("co000003", "order id required")
)

func (s *Service) GetCountriesList(
	ctx context.Context,
	req *billingpb.EmptyRequest,
	res *billingpb.CountriesList,
) error {
	countries, err := s.country.GetAll(ctx)
	if err != nil {
		return err
	}

	res.Countries = countries.Countries

	return nil
}

func (s *Service) GetCountriesListForOrder(
	ctx context.Context,
	req *billingpb.GetCountriesListForOrderRequest,
	res *billingpb.GetCountriesListForOrderResponse,
) error {
	if req.OrderId == "" {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorCountryOrderIdRequired
		return nil
	}

	order, err := s.orderRepository.GetByUuid(ctx, req.OrderId)
	if err != nil {
		zap.L().Error(pkg.MethodFinishedWithError, zap.Error(err))

		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = e
			return nil
		}
		return err
	}

	countries, err := s.country.FindByHighRisk(ctx, order.IsHighRisk)
	if err != nil {
		return err
	}

	res.Item = countries
	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetCountry(
	ctx context.Context,
	req *billingpb.GetCountryRequest,
	res *billingpb.Country,
) error {
	country, err := s.country.GetByIsoCodeA2(ctx, req.IsoCode)
	if err != nil {
		return err
	}
	res.IsoCodeA2 = country.IsoCodeA2
	res.Region = country.Region
	res.Currency = country.Currency
	res.PaymentsAllowed = country.PaymentsAllowed
	res.ChangeAllowed = country.ChangeAllowed
	res.VatEnabled = country.VatEnabled
	res.VatCurrency = country.VatCurrency
	res.PriceGroupId = country.PriceGroupId
	res.VatThreshold = country.VatThreshold
	res.VatPeriodMonth = country.VatPeriodMonth
	res.VatDeadlineDays = country.VatDeadlineDays
	res.VatStoreYears = country.VatStoreYears
	res.VatCurrencyRatesPolicy = country.VatCurrencyRatesPolicy
	res.VatCurrencyRatesSource = country.VatCurrencyRatesSource
	res.CreatedAt = country.CreatedAt
	res.UpdatedAt = country.UpdatedAt
	res.PayerTariffRegion = country.PayerTariffRegion
	res.HighRiskPaymentsAllowed = country.HighRiskPaymentsAllowed
	res.HighRiskChangeAllowed = country.HighRiskChangeAllowed

	return nil
}

func (s *Service) UpdateCountry(
	ctx context.Context,
	req *billingpb.Country,
	res *billingpb.Country,
) error {

	country, err := s.country.GetByIsoCodeA2(ctx, req.IsoCodeA2)
	if err != nil {
		return err
	}

	pg, err := s.priceGroupRepository.GetById(ctx, req.PriceGroupId)
	if err != nil {
		return err
	}

	var threshold *billingpb.CountryVatThreshold

	if req.VatThreshold != nil {
		threshold = req.VatThreshold
	} else {
		threshold = &billingpb.CountryVatThreshold{
			Year:  0,
			World: 0,
		}
	}

	update := &billingpb.Country{
		Id:                      country.Id,
		IsoCodeA2:               country.IsoCodeA2,
		Region:                  req.Region,
		Currency:                req.Currency,
		PaymentsAllowed:         req.PaymentsAllowed,
		ChangeAllowed:           req.ChangeAllowed,
		VatEnabled:              req.VatEnabled,
		VatCurrency:             req.VatCurrency,
		PriceGroupId:            pg.Id,
		VatThreshold:            threshold,
		VatPeriodMonth:          req.VatPeriodMonth,
		VatDeadlineDays:         req.VatDeadlineDays,
		VatStoreYears:           req.VatStoreYears,
		VatCurrencyRatesPolicy:  req.VatCurrencyRatesPolicy,
		VatCurrencyRatesSource:  req.VatCurrencyRatesSource,
		CreatedAt:               country.CreatedAt,
		UpdatedAt:               ptypes.TimestampNow(),
		HighRiskPaymentsAllowed: req.HighRiskPaymentsAllowed,
		HighRiskChangeAllowed:   req.HighRiskChangeAllowed,
	}

	err = s.country.Update(ctx, update)
	if err != nil {
		zap.S().Errorf("update country failed", "err", err.Error(), "data", update)
		return err
	}

	res.IsoCodeA2 = update.IsoCodeA2
	res.Region = update.Region
	res.Currency = update.Currency
	res.PaymentsAllowed = update.PaymentsAllowed
	res.ChangeAllowed = update.ChangeAllowed
	res.VatEnabled = update.VatEnabled
	res.VatCurrency = update.VatCurrency
	res.PriceGroupId = update.PriceGroupId
	res.VatThreshold = update.VatThreshold
	res.VatPeriodMonth = update.VatPeriodMonth
	res.VatDeadlineDays = update.VatDeadlineDays
	res.VatStoreYears = update.VatStoreYears
	res.VatCurrencyRatesPolicy = update.VatCurrencyRatesPolicy
	res.VatCurrencyRatesSource = update.VatCurrencyRatesSource
	res.CreatedAt = update.CreatedAt
	res.UpdatedAt = update.UpdatedAt
	res.HighRiskPaymentsAllowed = update.HighRiskPaymentsAllowed
	res.HighRiskChangeAllowed = update.HighRiskChangeAllowed

	return nil
}
