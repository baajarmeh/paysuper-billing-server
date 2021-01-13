package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"net/http"
)

var (
	projectErrorUnknown                                          = errors.NewBillingServerErrorMsg("pr000001", "project unknown error")
	projectErrorNotFound                                         = errors.NewBillingServerErrorMsg("pr000002", "project with specified identifier not found")
	projectErrorNameDefaultLangRequired                          = errors.NewBillingServerErrorMsg("pr000003", "project name in \""+DefaultLanguage+"\" locale is required")
	projectErrorCallbackCurrencyIncorrect                        = errors.NewBillingServerErrorMsg("pr000004", "project callback currency is incorrect")
	projectErrorLimitCurrencyIncorrect                           = errors.NewBillingServerErrorMsg("pr000005", "project limit currency is incorrect")
	projectErrorLimitCurrencyRequired                            = errors.NewBillingServerErrorMsg("pr000006", "project limit currency can't be empty if you send min or max payment amount")
	projectErrorCurrencyIsNotSupport                             = errors.NewBillingServerErrorMsg("pr000007", `project currency is not supported`)
	projectErrorVirtualCurrencyNameDefaultLangRequired           = errors.NewBillingServerErrorMsg("pr000008", "project virtual currency name in \""+DefaultLanguage+"\" locale is required")
	projectErrorVirtualCurrencySuccessMessageDefaultLangRequired = errors.NewBillingServerErrorMsg("pr000009", "project virtual currency success message in \""+DefaultLanguage+"\" locale is required")
	projectErrorVirtualCurrencyPriceCurrencyIsNotSupport         = errors.NewBillingServerErrorMsg("pr000010", `project virtual currency price currency is not support`)
	projectErrorVirtualCurrencyLimitsIncorrect                   = errors.NewBillingServerErrorMsg("pr000011", `project virtual currency purchase limits is incorrect`)
	projectErrorShortDescriptionDefaultLangRequired              = errors.NewBillingServerErrorMsg("pr000012", "project short description in \""+DefaultLanguage+"\" locale is required")
	projectErrorFullDescriptionDefaultLangRequired               = errors.NewBillingServerErrorMsg("pr000013", "project full description in \""+DefaultLanguage+"\" locale is required")
	projectErrorVatPayerUnknown                                  = errors.NewBillingServerErrorMsg("pr000014", "project vat payer unknown")
	projectErrorRedirectModeSuccessfulUrlIsRequired              = errors.NewBillingServerErrorMsg("pr000015", "redirect url for user's redirect after payment successful ending for selected redirect mode is required")
	projectErrorRedirectModeFailUrlIsRequired                    = errors.NewBillingServerErrorMsg("pr000016", "redirect url for user's redirect after failed payment for selected redirect mode is required")
	projectErrorRedirectModeBothRedirectUrlsIsRequired           = errors.NewBillingServerErrorMsg("pr000017", "redirect urls for user's redirect after payment completed for selected redirect mode is required")
	projectErrorButtonCaptionAllowedOnlyForAfterRedirect         = errors.NewBillingServerErrorMsg("pr000018", "caption for redirect button can't be set with non zero delay for auto redirect")
	projectErrorRedirectModeIsRequired                           = errors.NewBillingServerErrorMsg("pr000019", "redirect mode must be selected")
	projectErrorRedirectUsageIsRequired                          = errors.NewBillingServerErrorMsg("pr000020", "type of redirect usage must be selected")
	projectErrorChangeStatusToProductionNotAllowed               = errors.NewBillingServerErrorMsg("pr000021", "project status change to production not allowed, because project's merchant onboarding not complete")
)

func (s *Service) ChangeProject(
	ctx context.Context,
	req *billingpb.Project,
	rsp *billingpb.ChangeProjectResponse,
) error {
	var project *billingpb.Project
	merchant, err := s.merchantRepository.GetById(ctx, req.MerchantId)

	if err != nil {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound
		return nil
	}

	if req.Id != "" {
		project, err = s.project.GetById(ctx, req.Id)

		if err != nil || project.MerchantId != req.MerchantId {
			rsp.Status = billingpb.ResponseStatusNotFound
			rsp.Message = projectErrorNotFound
			return nil
		}
	}

	if _, ok := req.Name[DefaultLanguage]; !ok {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = projectErrorNameDefaultLangRequired

		return nil
	}

	if req.CallbackCurrency != "" {
		if !helper.Contains(s.supportedCurrencies, req.CallbackCurrency) {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = projectErrorCallbackCurrencyIncorrect

			return nil
		}
	}

	if req.LimitsCurrency != "" {
		if !helper.Contains(s.supportedCurrencies, req.LimitsCurrency) {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = projectErrorLimitCurrencyIncorrect

			return nil
		}
	}

	if len(req.Currencies) > 0 {
		for _, v := range req.Currencies {
			if !helper.Contains(s.supportedCurrencies, v.Currency) {
				rsp.Status = billingpb.ResponseStatusBadData
				rsp.Message = projectErrorCurrencyIsNotSupport
				rsp.Message.Details = v.Currency

				return nil
			}
		}
	}

	if req.VirtualCurrency != nil {
		payoutCurrency := merchant.GetPayoutCurrency()
		err = s.validateProjectVirtualCurrency(req.VirtualCurrency, payoutCurrency)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = err.(*billingpb.ResponseErrorMessage)

			return nil
		}

		if req.VirtualCurrency.SellCountType == "" {
			req.VirtualCurrency.SellCountType = pkg.ProjectSellCountTypeFractional
		}
	}

	if len(req.ShortDescription) > 0 {
		if _, ok := req.ShortDescription[DefaultLanguage]; !ok {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = projectErrorShortDescriptionDefaultLangRequired

			return nil
		}
	}

	if len(req.FullDescription) > 0 {
		if _, ok := req.FullDescription[DefaultLanguage]; !ok {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = projectErrorFullDescriptionDefaultLangRequired

			return nil
		}
	}

	if (req.MinPaymentAmount > 0 || req.MaxPaymentAmount > 0) && req.LimitsCurrency == "" {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = projectErrorLimitCurrencyRequired

		return nil
	}

	if merchant.DontChargeVat == true {
		req.VatPayer = billingpb.VatPayerNobody
	} else {
		if req.VatPayer != billingpb.VatPayerBuyer && req.VatPayer != billingpb.VatPayerSeller {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = projectErrorVatPayerUnknown

			return nil
		}
	}

	if req.RedirectSettings != nil {
		err = s.validateRedirectSettings(req)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = err.(*billingpb.ResponseErrorMessage)
			return nil
		}
	} else {
		if project == nil {
			req.RedirectSettings = &billingpb.ProjectRedirectSettings{
				Mode:  pkg.ProjectRedirectModeDisable,
				Usage: pkg.ProjectRedirectUsageAny,
			}
		}
	}

	if req.Status == billingpb.ProjectStatusInProduction && (!merchant.IsDataComplete() || !merchant.IsSigned) {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = projectErrorChangeStatusToProductionNotAllowed
		return nil
	}

	if project == nil {
		project, err = s.createProject(ctx, req)
	} else {
		err = s.updateProject(ctx, req, project)
	}

	if err != nil {
		rsp.Status = billingpb.ResponseStatusSystemError
		zap.S().Errorw("create or update project error", "err", err, "req", req)
		rsp.Message = projectErrorUnknown

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = project

	return nil
}

func (s *Service) GetProject(
	ctx context.Context,
	req *billingpb.GetProjectRequest,
	rsp *billingpb.ChangeProjectResponse,
) error {
	project, err := s.project.GetById(ctx, req.ProjectId)

	if err != nil || (req.MerchantId != "" && project.MerchantId != req.MerchantId) {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = projectErrorNotFound
		return nil
	}

	project.ProductsCount, _ = s.productRepository.CountByProject(ctx, project.Id)

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = project

	return nil
}

func (s *Service) ListProjects(
	ctx context.Context,
	req *billingpb.ListProjectsRequest,
	rsp *billingpb.ListProjectsResponse,
) error {
	count, err := s.project.FindCount(ctx, req.MerchantId, req.QuickSearch, req.Statuses)

	if err != nil {
		return projectErrorUnknown
	}

	projects, err := s.project.Find(
		ctx,
		req.MerchantId,
		req.QuickSearch,
		req.Statuses,
		int64(req.Offset),
		int64(req.Limit),
		req.Sort,
	)

	if err != nil {
		return projectErrorUnknown
	}

	rsp.Count = count
	rsp.Items = []*billingpb.Project{}

	if count > 0 {
		rsp.Items = projects
	}

	return nil
}

func (s *Service) DeleteProject(
	ctx context.Context,
	req *billingpb.GetProjectRequest,
	rsp *billingpb.ChangeProjectResponse,
) error {
	project, err := s.project.GetById(ctx, req.ProjectId)

	if err != nil || req.MerchantId != project.MerchantId {
		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = projectErrorNotFound

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk

	if project.IsDeleted() == true {
		return nil
	}

	project.Status = billingpb.ProjectStatusDeleted

	if err := s.project.Update(ctx, project); err != nil {
		zap.S().Errorf("Query to delete project failed", "err", err.Error(), "data", project)

		rsp.Status = billingpb.ResponseStatusSystemError
		rsp.Message = projectErrorUnknown

		return nil
	}

	return nil
}

func (s *Service) createProject(ctx context.Context, req *billingpb.Project) (*billingpb.Project, error) {
	project := &billingpb.Project{
		Id:                       primitive.NewObjectID().Hex(),
		MerchantId:               req.MerchantId,
		Cover:                    req.Cover,
		Name:                     req.Name,
		CallbackCurrency:         req.CallbackCurrency,
		CallbackProtocol:         req.CallbackProtocol,
		CreateOrderAllowedUrls:   req.CreateOrderAllowedUrls,
		AllowDynamicNotifyUrls:   req.AllowDynamicNotifyUrls,
		AllowDynamicRedirectUrls: req.AllowDynamicRedirectUrls,
		LimitsCurrency:           req.LimitsCurrency,
		MinPaymentAmount:         req.MinPaymentAmount,
		MaxPaymentAmount:         req.MaxPaymentAmount,
		NotifyEmails:             req.NotifyEmails,
		IsProductsCheckout:       req.IsProductsCheckout,
		SecretKey:                req.SecretKey,
		SignatureRequired:        req.SignatureRequired,
		SendNotifyEmail:          req.SendNotifyEmail,
		UrlCheckAccount:          req.UrlCheckAccount,
		UrlProcessPayment:        req.UrlProcessPayment,
		UrlRedirectFail:          req.UrlRedirectFail,
		UrlRedirectSuccess:       req.UrlRedirectSuccess,
		UrlChargebackPayment:     req.UrlChargebackPayment,
		UrlCancelPayment:         req.UrlCancelPayment,
		UrlFraudPayment:          req.UrlFraudPayment,
		UrlRefundPayment:         req.UrlRefundPayment,
		Status:                   billingpb.ProjectStatusDraft,
		Localizations:            req.Localizations,
		FullDescription:          req.FullDescription,
		ShortDescription:         req.ShortDescription,
		Currencies:               req.Currencies,
		VirtualCurrency:          req.VirtualCurrency,
		VatPayer:                 req.VatPayer,
		RedirectSettings:         req.RedirectSettings,
		CreatedAt:                ptypes.TimestampNow(),
		UpdatedAt:                ptypes.TimestampNow(),
	}

	if err := s.project.Insert(ctx, project); err != nil {
		zap.S().Errorf("Query to create project failed", "err", err.Error(), "data", project)
		return nil, projectErrorUnknown
	}

	return project, nil
}

func (s *Service) updateProject(ctx context.Context, req *billingpb.Project, project *billingpb.Project) error {
	project.Name = req.Name
	project.CallbackCurrency = req.CallbackCurrency
	project.CreateOrderAllowedUrls = req.CreateOrderAllowedUrls
	project.AllowDynamicNotifyUrls = req.AllowDynamicNotifyUrls
	project.AllowDynamicRedirectUrls = req.AllowDynamicRedirectUrls
	project.LimitsCurrency = req.LimitsCurrency
	project.MinPaymentAmount = req.MinPaymentAmount
	project.MaxPaymentAmount = req.MaxPaymentAmount
	project.NotifyEmails = req.NotifyEmails
	project.IsProductsCheckout = req.IsProductsCheckout
	project.SecretKey = req.SecretKey
	project.SignatureRequired = req.SignatureRequired
	project.SendNotifyEmail = req.SendNotifyEmail
	project.UrlRedirectFail = req.UrlRedirectFail
	project.UrlRedirectSuccess = req.UrlRedirectSuccess
	project.Status = req.Status
	project.UpdatedAt = ptypes.TimestampNow()
	project.CallbackProtocol = req.CallbackProtocol
	project.UrlCheckAccount = req.UrlCheckAccount
	project.UrlProcessPayment = req.UrlProcessPayment
	project.UrlChargebackPayment = req.UrlChargebackPayment
	project.UrlCancelPayment = req.UrlCancelPayment
	project.UrlFraudPayment = req.UrlFraudPayment
	project.UrlRefundPayment = req.UrlRefundPayment
	project.Localizations = req.Localizations
	project.FullDescription = req.FullDescription
	project.ShortDescription = req.ShortDescription
	project.Currencies = req.Currencies
	project.VirtualCurrency = req.VirtualCurrency
	project.VatPayer = req.VatPayer
	project.Cover = req.Cover

	if req.RedirectSettings != nil {
		project.RedirectSettings = req.RedirectSettings
	}

	if req.WebhookMode != "" {
		project.WebhookMode = req.WebhookMode
	}

	if err := s.project.Update(ctx, project); err != nil {
		return projectErrorUnknown
	}

	project.ProductsCount, _ = s.productRepository.CountByProject(ctx, project.Id)

	return nil
}

func (s *Service) validateProjectVirtualCurrency(virtualCurrency *billingpb.ProjectVirtualCurrency, payoutCurrency string) error {
	if _, ok := virtualCurrency.Name[DefaultLanguage]; !ok {
		return projectErrorVirtualCurrencyNameDefaultLangRequired
	}

	if _, ok := virtualCurrency.SuccessMessage[DefaultLanguage]; !ok {
		return projectErrorVirtualCurrencySuccessMessageDefaultLangRequired
	}

	if len(virtualCurrency.Prices) > 0 {
		currencies := make([]string, len(virtualCurrency.Prices))

		for _, v := range virtualCurrency.Prices {
			if !helper.Contains(s.supportedCurrencies, v.Currency) {
				err := projectErrorVirtualCurrencyPriceCurrencyIsNotSupport
				err.Details = v.Currency

				return err
			}
			currencies = append(currencies, v.Currency)
		}

		if !helper.Contains(currencies, payoutCurrency) {
			err := projectErrorVirtualCurrencyPriceCurrencyIsNotSupport
			err.Details = payoutCurrency

			return err
		}
	}

	if virtualCurrency.MinPurchaseValue > 0 && virtualCurrency.MaxPurchaseValue > 0 &&
		virtualCurrency.MinPurchaseValue > virtualCurrency.MaxPurchaseValue {
		return projectErrorVirtualCurrencyLimitsIncorrect
	}

	return nil
}

// Validate redirect settings for project in request to project create or update
func (s *Service) validateRedirectSettings(req *billingpb.Project) error {
	settings := req.RedirectSettings

	if settings.Mode == "" {
		return projectErrorRedirectModeIsRequired
	}

	if settings.Usage == "" {
		return projectErrorRedirectUsageIsRequired
	}

	if settings.Mode == pkg.ProjectRedirectModeSuccessful && req.UrlRedirectSuccess == "" {
		return projectErrorRedirectModeSuccessfulUrlIsRequired
	}

	if settings.Mode == pkg.ProjectRedirectModeFail && req.UrlRedirectFail == "" {
		return projectErrorRedirectModeFailUrlIsRequired
	}

	if settings.Mode == pkg.ProjectRedirectModeAny && (req.UrlRedirectFail == "" || req.UrlRedirectSuccess == "") {
		return projectErrorRedirectModeBothRedirectUrlsIsRequired
	}

	if settings.Delay > 0 && settings.ButtonCaption != "" {
		return projectErrorButtonCaptionAllowedOnlyForAfterRedirect
	}

	return nil
}

func (s *Service) CheckSkuAndKeyProject(ctx context.Context, req *billingpb.CheckSkuAndKeyProjectRequest, rsp *billingpb.EmptyResponseWithStatus) error {
	rsp.Status = billingpb.ResponseStatusOk

	found, err := s.keyProductRepository.CountByProjectIdSku(ctx, req.ProjectId, req.Sku)

	if err != nil {
		rsp.Status = http.StatusBadRequest
		rsp.Message = keyProductRetrieveError
		return nil
	}

	if found > 0 {
		rsp.Status = http.StatusBadRequest
		rsp.Message = keyProductDuplicate
		return nil
	}

	return nil
}
