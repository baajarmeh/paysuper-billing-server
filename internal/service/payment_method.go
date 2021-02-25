package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.uber.org/zap"
	"strings"
)

const (
	paymentMethodErrorPaymentSystem              = "payment method must contain of payment system"
	paymentMethodErrorUnknownMethod              = "payment method is unknown"
	paymentMethodErrorNotFoundProductionSettings = "payment method is not contain requesting settings"
)

func (s *Service) CreateOrUpdatePaymentMethod(
	ctx context.Context,
	req *billingpb.PaymentMethod,
	rsp *billingpb.ChangePaymentMethodResponse,
) error {
	var pm *billingpb.PaymentMethod
	var err error

	if _, err = s.paymentSystemRepository.GetById(ctx, req.PaymentSystemId); err != nil {
		zap.S().Errorf("Invalid payment system id for update payment method", "err", err.Error(), "data", req)
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = paymentMethodErrorPaymentSystem

		return nil
	}

	if req.Id != "" {
		pm, err = s.paymentMethodRepository.GetById(ctx, req.Id)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = paymentMethodErrorUnknownMethod

			return nil
		}
	}

	if req.IsActive == true && req.IsValid() == false {
		zap.S().Errorf("Set all parameters of the payment method before its activation", "data", req)
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = paymentMethodErrorPaymentSystem

		return nil
	}

	req.UpdatedAt = ptypes.TimestampNow()

	if pm == nil {
		req.CreatedAt = ptypes.TimestampNow()
		err = s.paymentMethodRepository.Insert(ctx, req)
	} else {
		pm.ExternalId = req.ExternalId
		pm.TestSettings = req.TestSettings
		pm.ProductionSettings = req.ProductionSettings
		pm.Name = req.Name
		pm.IsActive = req.IsActive
		pm.Group = req.Group
		pm.Type = req.Type
		pm.AccountRegexp = req.AccountRegexp
		pm.LimitsCurrency = req.LimitsCurrency
		pm.MaxPaymentAmount = req.MaxPaymentAmount
		pm.MinPaymentAmount = req.MinPaymentAmount
		err = s.paymentMethodRepository.Update(ctx, pm)
	}

	if err != nil {
		zap.S().Errorf("Query to insert|update project method is failed", "err", err.Error(), "data", req)
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) CreateOrUpdatePaymentMethodProductionSettings(
	ctx context.Context,
	req *billingpb.ChangePaymentMethodParamsRequest,
	rsp *billingpb.ChangePaymentMethodParamsResponse,
) error {
	var pm *billingpb.PaymentMethod
	var err error

	pm, err = s.paymentMethodRepository.GetById(ctx, req.PaymentMethodId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = paymentMethodErrorUnknownMethod

		return nil
	}

	if pm.ProductionSettings == nil {
		pm.ProductionSettings = map[string]*billingpb.PaymentMethodParams{}
	}

	brands := []string{}
	for _, brand := range req.Params.Brand {
		brands = append(brands, strings.ToUpper(brand))
	}

	settings := &billingpb.PaymentMethodParams{
		Currency:           strings.ToUpper(req.Params.Currency),
		Secret:             req.Params.Secret,
		SecretCallback:     req.Params.SecretCallback,
		TerminalId:         req.Params.TerminalId,
		MccCode:            req.Params.MccCode,
		OperatingCompanyId: strings.ToLower(req.Params.OperatingCompanyId),
		Brand:              brands,
	}

	key := helper.GetPaymentMethodKey(req.Params.Currency, req.Params.MccCode, req.Params.OperatingCompanyId, "")
	pm.ProductionSettings[key] = settings

	for _, brand := range req.Params.Brand {
		key := helper.GetPaymentMethodKey(req.Params.Currency, req.Params.MccCode, req.Params.OperatingCompanyId, brand)
		pm.ProductionSettings[key] = settings
	}

	if err := s.paymentMethodRepository.Update(ctx, pm); err != nil {
		zap.S().Errorf("Query to update production settings of project method is failed", "err", err.Error(), "data", req)
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetPaymentMethodProductionSettings(
	ctx context.Context,
	req *billingpb.GetPaymentMethodSettingsRequest,
	rsp *billingpb.GetPaymentMethodSettingsResponse,
) error {
	pm, err := s.paymentMethodRepository.GetById(ctx, req.PaymentMethodId)
	if err != nil {
		return nil
	}

	check := make(map[string]bool)

	for _, param := range pm.ProductionSettings {

		key := helper.GetPaymentMethodKey(param.Currency, param.MccCode, param.OperatingCompanyId, "")

		if check[key] == true {
			continue
		}

		check[key] = true

		rsp.Params = append(rsp.Params, &billingpb.PaymentMethodParams{
			Currency:           param.Currency,
			TerminalId:         param.TerminalId,
			Secret:             param.Secret,
			SecretCallback:     param.SecretCallback,
			MccCode:            param.MccCode,
			OperatingCompanyId: param.OperatingCompanyId,
			Brand:              param.Brand,
		})
	}

	return nil
}

func (s *Service) DeletePaymentMethodProductionSettings(
	ctx context.Context,
	req *billingpb.GetPaymentMethodSettingsRequest,
	rsp *billingpb.ChangePaymentMethodParamsResponse,
) error {
	pm, err := s.paymentMethodRepository.GetById(ctx, req.PaymentMethodId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = paymentMethodErrorUnknownMethod

		return nil
	}

	deleteKeys := []string{}

	key := helper.GetPaymentMethodKey(req.CurrencyA3, req.MccCode, req.OperatingCompanyId, "")
	deleteKeys = append(deleteKeys, key)

	setting, ok := pm.ProductionSettings[key]
	if !ok {
		zap.S().Errorf("Unable to get production settings for currency", "data", req)
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = paymentMethodErrorNotFoundProductionSettings

		return nil
	}

	for _, brand := range setting.Brand {
		key := helper.GetPaymentMethodKey(req.CurrencyA3, req.MccCode, req.OperatingCompanyId, brand)
		deleteKeys = append(deleteKeys, key)
	}

	for _, key := range deleteKeys {
		delete(pm.ProductionSettings, key)
	}

	if err := s.paymentMethodRepository.Update(ctx, pm); err != nil {
		zap.S().Errorf("Query to delete production settings of project method is failed", "err", err.Error(), "data", req)
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) CreateOrUpdatePaymentMethodTestSettings(
	ctx context.Context,
	req *billingpb.ChangePaymentMethodParamsRequest,
	rsp *billingpb.ChangePaymentMethodParamsResponse,
) error {
	var pm *billingpb.PaymentMethod
	var err error

	pm, err = s.paymentMethodRepository.GetById(ctx, req.PaymentMethodId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = paymentMethodErrorUnknownMethod

		return nil
	}

	if pm.TestSettings == nil {
		pm.TestSettings = map[string]*billingpb.PaymentMethodParams{}
	}

	brands := []string{}
	for _, brand := range req.Params.Brand {
		brands = append(brands, strings.ToUpper(brand))
	}

	settings := &billingpb.PaymentMethodParams{
		Currency:           strings.ToUpper(req.Params.Currency),
		Secret:             req.Params.Secret,
		SecretCallback:     req.Params.SecretCallback,
		TerminalId:         req.Params.TerminalId,
		MccCode:            req.Params.MccCode,
		OperatingCompanyId: strings.ToLower(req.Params.OperatingCompanyId),
		Brand:              brands,
	}

	key := helper.GetPaymentMethodKey(req.Params.Currency, req.Params.MccCode, req.Params.OperatingCompanyId, "")
	pm.TestSettings[key] = settings

	for _, brand := range req.Params.Brand {
		key := helper.GetPaymentMethodKey(req.Params.Currency, req.Params.MccCode, req.Params.OperatingCompanyId, brand)
		pm.TestSettings[key] = settings
	}

	if err := s.paymentMethodRepository.Update(ctx, pm); err != nil {
		zap.S().Errorf("Query to update production settings of project method is failed", "err", err.Error(), "data", req)
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetPaymentMethodTestSettings(
	ctx context.Context,
	req *billingpb.GetPaymentMethodSettingsRequest,
	rsp *billingpb.GetPaymentMethodSettingsResponse,
) error {
	pm, err := s.paymentMethodRepository.GetById(ctx, req.PaymentMethodId)
	if err != nil {
		return nil
	}

	check := make(map[string]bool)

	for _, param := range pm.TestSettings {

		key := helper.GetPaymentMethodKey(param.Currency, param.MccCode, param.OperatingCompanyId, "")

		if check[key] == true {
			continue
		}

		check[key] = true

		rsp.Params = append(rsp.Params, &billingpb.PaymentMethodParams{
			Currency:           param.Currency,
			TerminalId:         param.TerminalId,
			Secret:             param.Secret,
			SecretCallback:     param.SecretCallback,
			MccCode:            param.MccCode,
			OperatingCompanyId: param.OperatingCompanyId,
			Brand:              param.Brand,
		})
	}

	return nil
}

func (s *Service) DeletePaymentMethodTestSettings(
	ctx context.Context,
	req *billingpb.GetPaymentMethodSettingsRequest,
	rsp *billingpb.ChangePaymentMethodParamsResponse,
) error {
	pm, err := s.paymentMethodRepository.GetById(ctx, req.PaymentMethodId)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = paymentMethodErrorUnknownMethod

		return nil
	}

	deleteKeys := []string{}

	key := helper.GetPaymentMethodKey(req.CurrencyA3, req.MccCode, req.OperatingCompanyId, "")
	deleteKeys = append(deleteKeys, key)

	setting, ok := pm.TestSettings[key]
	if !ok {
		zap.S().Errorf("Unable to get production settings for currency", "data", req)
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = paymentMethodErrorNotFoundProductionSettings

		return nil
	}

	for _, brand := range setting.Brand {
		key := helper.GetPaymentMethodKey(req.CurrencyA3, req.MccCode, req.OperatingCompanyId, brand)
		deleteKeys = append(deleteKeys, key)
	}

	for _, key := range deleteKeys {
		delete(pm.ProductionSettings, key)
	}

	if err := s.paymentMethodRepository.Update(ctx, pm); err != nil {
		zap.S().Errorf("Query to delete production settings of project method is failed", "err", err.Error(), "data", req)
		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) getPaymentSettings(
	paymentMethod *billingpb.PaymentMethod,
	currency string,
	mccCode string,
	operatingCompanyId string,
	paymentMethodBrand string,
	isProduction bool,
) (*billingpb.PaymentMethodParams, error) {
	settings := paymentMethod.TestSettings

	if isProduction == true {
		settings = paymentMethod.ProductionSettings
	}

	if settings == nil {
		return nil, orderErrorPaymentMethodEmptySettings
	}

	key := helper.GetPaymentMethodKey(currency, mccCode, operatingCompanyId, paymentMethodBrand)

	setting, ok := settings[key]

	if !ok || !setting.IsSettingComplete() {
		return nil, orderErrorPaymentMethodEmptySettings
	}

	setting.Currency = currency

	if isProduction == true {
		setting.ApiUrl = s.cfg.CardPayApiUrl
	} else {
		setting.ApiUrl = s.cfg.CardPayApiSandboxUrl
	}

	return setting, nil
}
