package service

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"net/http"
	"sort"
	"strings"
)

const (
	oneDayTtl                 = 86400
	defaultMinimalLimitNotify = 100
)

var (
	keyProductMerchantMismatch              = errors.NewBillingServerErrorMsg("kp000001", "merchant id mismatch")
	keyProductProjectMismatch               = errors.NewBillingServerErrorMsg("kp000002", "project id mismatch")
	keyProductSkuMismatch                   = errors.NewBillingServerErrorMsg("kp000003", "sku mismatch")
	keyProductNameNotProvided               = errors.NewBillingServerErrorMsg("kp000004", "name must be set")
	keyProductDescriptionNotProvided        = errors.NewBillingServerErrorMsg("kp000005", "description must be set")
	keyProductDuplicate                     = errors.NewBillingServerErrorMsg("kp000006", "sku+project id already exist")
	keyProductIdsIsEmpty                    = errors.NewBillingServerErrorMsg("kp000007", "ids is empty")
	keyProductAlreadyHasPlatform            = errors.NewBillingServerErrorMsg("kp000008", "product already has user defined platform")
	keyProductActivationUrlEmpty            = errors.NewBillingServerErrorMsg("kp000009", "activation url must be set")
	keyProductEulaEmpty                     = errors.NewBillingServerErrorMsg("kp000010", "eula url must be set")
	keyProductPlatformName                  = errors.NewBillingServerErrorMsg("kp000011", "platform name must be set")
	keyProductRetrieveError                 = errors.NewBillingServerErrorMsg("kp000012", "query to retrieve key product failed")
	keyProductErrorUpsert                   = errors.NewBillingServerErrorMsg("kp000013", "query to insert/update key product failed")
	keyProductErrorDelete                   = errors.NewBillingServerErrorMsg("kp000014", "query to remove key product failed")
	keyProductMerchantNotFound              = errors.NewBillingServerErrorMsg("kp000015", "merchant not found")
	keyProductNotFound                      = errors.NewBillingServerErrorMsg("kp000017", "key product not found")
	keyProductInternalError                 = errors.NewBillingServerErrorMsg("kp000018", "unknown error")
	keyProductOrderIsNotProcessedError      = errors.NewBillingServerErrorMsg("kp000019", "order has wrong public status")
	keyProductPlatformDontHaveDefaultPrice  = errors.NewBillingServerErrorMsg("kp000020", "platform don't have price in default currency")
	keyProductPlatformPriceMismatchCurrency = errors.NewBillingServerErrorMsg("kp000021", "platform don't have price with region that mismatch with currency")
	keyProductNotPublished                  = errors.NewBillingServerErrorMsg("kp000023", "key product is not published")
)

var availablePlatforms = map[string]*billingpb.Platform{
	"steam": {
		Id:                       "steam",
		Name:                     "Steam",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-steam.png",
		Order:                    1,
		ActivationInstructionUrl: "https://support.steampowered.com/kb_article.php?ref=7480-WUSF-3601&l=english",
	},
	"gog": {
		Id:                       "gog",
		Name:                     "GOG",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-gog.png",
		Order:                    2,
		ActivationInstructionUrl: "https://www.gog.com/redeem",
	},
	"uplay": {
		Id:                       "uplay",
		Name:                     "Uplay",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-uplay.png",
		Order:                    3,
		ActivationInstructionUrl: "https://support.ubi.com/en-us/Faqs/000025121/How-to-redeem-keys-codes",
	},
	"origin": {
		Id:                       "origin",
		Name:                     "Origin",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-origin.png",
		Order:                    4,
		ActivationInstructionUrl: "https://help.ea.com/en-us/help/origin/origin/origin-code-redemption-faq/#redeemcode",
	},
	"psn": {
		Id:                       "psn",
		Name:                     "PSN",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-psn.png",
		Order:                    5,
		ActivationInstructionUrl: "https://id.sonyentertainmentnetwork.com/id/management/#/p/commerce/content/redeem_code",
	},
	"xbox": {
		Id:                       "xbox",
		Name:                     "XBOX Store",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-xbox.png",
		Order:                    6,
		ActivationInstructionUrl: "https://redeem.microsoft.com",
	},
	"nintendo": {
		Id:                       "nintendo",
		Name:                     "Nintendo Store",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-nintendo.png",
		Order:                    7,
		ActivationInstructionUrl: "https://ec.nintendo.com/redeem/#/",
	},
	"itch": {
		Id:                       "itch",
		Name:                     "Itch.io",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-itch.png",
		Order:                    8,
		ActivationInstructionUrl: "https://itch.io/docs/creators/download-keys",
	},
	"egs": {
		Id:                       "egs",
		Name:                     "Epic Games Store",
		Icon:                     "https://cdn.pay.super.com/img/logo-platforms/logo-epic.png",
		Order:                    9,
		ActivationInstructionUrl: "https://www.epicgames.com/store/en-US/redeem",
	},
}

func (s *Service) CreateOrUpdateKeyProduct(
	ctx context.Context,
	req *billingpb.CreateOrUpdateKeyProductRequest,
	res *billingpb.KeyProductResponse,
) error {
	var (
		err     error
		isNew   = len(req.Id) == 0
		now     = ptypes.TimestampNow()
		product = &billingpb.KeyProduct{}
	)
	res.Status = billingpb.ResponseStatusOk
	project, err := s.project.GetById(ctx, req.ProjectId)

	if err != nil {
		zap.S().Errorw("internal error when getting project", "err", err)
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = keyProductInternalError
		return nil
	}

	if project.MerchantId != req.MerchantId {
		zap.S().Errorw("Merchant for project is mismatch with requested", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductNotFound
		return nil
	}

	if isNew {
		product.Id = primitive.NewObjectID().Hex()
		product.CreatedAt = now
		product.MerchantId = req.MerchantId
		product.ProjectId = req.ProjectId
		product.Sku = req.Sku
	} else {
		productResponse := &billingpb.KeyProductResponse{}
		err = s.GetKeyProduct(ctx, &billingpb.RequestKeyProductMerchant{Id: req.Id, MerchantId: req.MerchantId}, productResponse)
		if err != nil {
			zap.S().Errorf("internal error when getting product", "err", err)
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = keyProductInternalError
			return nil
		}

		product = productResponse.Product

		if productResponse.Status != billingpb.ResponseStatusOk {
			zap.S().Errorf("failed to fetch key product", "message", productResponse.Message, "req", req)
			res.Status = productResponse.Status
			res.Message = productResponse.Message
			return nil
		}

		if req.Sku != "" && req.Sku != product.Sku {
			zap.S().Errorf("SKU mismatch", "data", req)
			res.Status = http.StatusBadRequest
			res.Message = keyProductSkuMismatch
			return nil
		}

		if req.MerchantId != product.MerchantId {
			zap.S().Errorf("MerchantId mismatch", "data", req)
			res.Status = http.StatusBadRequest
			res.Message = keyProductMerchantMismatch
			return nil
		}

		if req.ProjectId != product.ProjectId {
			zap.S().Errorf("ProjectId mismatch", "data", req)
			res.Status = http.StatusBadRequest
			res.Message = keyProductProjectMismatch
			return nil
		}
	}

	if _, ok := req.Name[DefaultLanguage]; !ok {
		zap.S().Errorf("No name in default language", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductNameNotProvided
		return nil
	}

	if _, ok := req.Description[DefaultLanguage]; !ok {
		zap.S().Errorf("No description in default language", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductDescriptionNotProvided
		return nil
	}

	// Prevent duplicated key products (by projectId+sku)
	found, err := s.keyProductRepository.CountByProjectIdSku(ctx, req.ProjectId, req.Sku)

	if err != nil {
		zap.S().Errorf("Query to find duplicates failed", "err", err.Error(), "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductRetrieveError
		return nil
	}

	allowed := int64(1)

	if isNew {
		allowed = 0
	}

	if found > allowed {
		zap.S().Errorf("Pair projectId+Sku already exists", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductDuplicate
		return nil
	}

	countUserDefinedPlatforms := 0

	merchant, err := s.merchantRepository.GetById(ctx, product.MerchantId)
	if err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = merchantErrorNotFound

		return nil
	}

	payoutCurrency := merchant.GetProcessingDefaultCurrency()
	if len(payoutCurrency) == 0 {
		zap.S().Errorw(merchantPayoutCurrencyMissed.Message, "data", req)
		res.Status = http.StatusBadRequest
		res.Message = merchantPayoutCurrencyMissed
		return nil
	}

	for _, platform := range req.Platforms {
		available, ok := availablePlatforms[platform.Id]
		if !ok {
			countUserDefinedPlatforms++
			if countUserDefinedPlatforms > 1 {
				zap.S().Errorw("Product has more that 1 user defined platforms", "data", req)
				res.Status = http.StatusBadRequest
				res.Message = keyProductAlreadyHasPlatform
				return nil
			}

			if platform.ActivationUrl == "" {
				zap.S().Errorw("Activation url must be set", "data", req)
				res.Status = http.StatusBadRequest
				res.Message = keyProductActivationUrlEmpty
				return nil
			}

			if platform.EulaUrl == "" {
				zap.S().Errorw("Eula url must be set", "data", req)
				res.Status = http.StatusBadRequest
				res.Message = keyProductEulaEmpty
				return nil
			}

			if platform.Name == "" {
				zap.S().Errorw("Name must be set", "data", req)
				res.Status = http.StatusBadRequest
				res.Message = keyProductPlatformName
				return nil
			}
		} else {
			platform.Name = available.Name
		}

		isHaveDefaultPrice := false

		// Check that user specified price in default currency
		for _, price := range platform.Prices {
			if price.Currency == payoutCurrency {
				isHaveDefaultPrice = true
			}

			pr, err := s.priceGroupRepository.GetByRegion(ctx, price.Region)
			if err != nil {
				zap.S().Errorw("Failed to get price group for region", "price", price)
				res.Status = billingpb.ResponseStatusBadData
				res.Message = keyProductInternalError
				return nil
			}

			if pr.Currency != price.Currency {
				zap.S().Errorw("Currency is mismatch for specified region", "price", price)
				res.Status = billingpb.ResponseStatusBadData
				res.Message = keyProductPlatformPriceMismatchCurrency
				res.Message.Details = fmt.Sprintf("price with regin `%s` should have currency `%s` but have `%s`", price.Region, pr.Currency, price.Currency)
				return nil
			}
		}

		if isHaveDefaultPrice == false {
			res.Status = http.StatusBadRequest
			res.Message = keyProductPlatformDontHaveDefaultPrice
			res.Message.Details = fmt.Sprintf("platform `%s` should have price in currency `%s`", platform.Id, req.DefaultCurrency)
			return nil
		}
	}

	product.Platforms = req.Platforms
	product.Metadata = req.Metadata
	product.Object = req.Object
	product.Name = req.Name
	product.DefaultCurrency = req.DefaultCurrency
	product.Description = req.Description
	product.LongDescription = req.LongDescription
	product.Cover = req.Cover
	product.Url = req.Url
	product.Pricing = req.Pricing
	product.UpdatedAt = now
	product.MinimalLimitNotify = req.MinimalLimitNotify

	if product.MinimalLimitNotify < 1 {
		product.MinimalLimitNotify = defaultMinimalLimitNotify
	}

	if err = s.keyProductRepository.Upsert(ctx, product); err != nil {
		res.Status = http.StatusInternalServerError
		res.Message = keyProductErrorUpsert
		return nil
	}

	res.Product = product
	return nil
}

func (s *Service) GetKeyProducts(
	ctx context.Context,
	req *billingpb.ListKeyProductsRequest,
	res *billingpb.ListKeyProductsResponse,
) error {
	res.Status = billingpb.ResponseStatusOk
	merchant, err := s.merchantRepository.GetById(ctx, req.MerchantId)

	if merchant == nil || err != nil {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = keyProductMerchantNotFound
		return nil
	}

	count, err := s.keyProductRepository.FindCount(ctx, req.MerchantId, req.ProjectId, req.Sku, req.Name, req.Enabled)

	if err != nil {
		res.Status = http.StatusInternalServerError
		res.Message = keyProductInternalError
		res.Message.Details = err.Error()
		return nil
	}

	res.Limit = req.Limit
	res.Offset = req.Offset
	res.Count = count
	res.Products = nil

	if res.Count == 0 || res.Offset > res.Count {
		return nil
	}

	items, err := s.keyProductRepository.Find(ctx, req.MerchantId, req.ProjectId, req.Sku, req.Name, req.Enabled, req.Offset, req.Limit)

	if err != nil {
		res.Status = http.StatusInternalServerError
		res.Message = keyProductInternalError.GetResponseErrorWithDetails(err.Error())
		return nil
	}

	for _, item := range items {
		for _, platform := range item.Platforms {
			keysRsp := &billingpb.GetPlatformKeyCountResponse{}
			err := s.GetAvailableKeysCount(ctx, &billingpb.GetPlatformKeyCountRequest{PlatformId: platform.Id, MerchantId: item.MerchantId, KeyProductId: item.Id}, keysRsp)
			if err != nil {
				zap.S().Errorw("Query to find count keys for platform failed", "err", err.Error(), "platform", platform.Id, "product.id", item.Id)
				res.Status = http.StatusInternalServerError
				res.Message = keyProductInternalError
				return nil
			}
			if keysRsp.Status != billingpb.ResponseStatusOk {
				zap.S().Errorw("Query to find count keys for platform failed", "message", keysRsp.Message, "platform", platform.Id, "product.id", item.Id)
				res.Status = keysRsp.Status
				res.Message = keysRsp.Message
				return nil
			}

			platform.Count = keysRsp.Count
		}
	}

	res.Products = items
	return nil
}

func (s *Service) GetKeyProductInfo(
	ctx context.Context,
	req *billingpb.GetKeyProductInfoRequest,
	res *billingpb.GetKeyProductInfoResponse,
) error {
	res.Status = billingpb.ResponseStatusOk
	product, err := s.keyProductRepository.GetById(ctx, req.KeyProductId)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			zap.S().Errorf("Key product not found", "id", req.KeyProductId)
			res.Status = http.StatusNotFound
			res.Message = keyProductNotFound
			return nil
		}

		zap.S().Errorf("Query to find key product by id failed", "err", err.Error(), "data", req)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductRetrieveError
		return nil
	}

	if !product.Enabled {
		zap.S().Error("Product is disabled", "data", req)
		res.Status = billingpb.ResponseStatusBadData
		res.Message = keyProductRetrieveError
		return nil
	}

	res.KeyProduct = &billingpb.KeyProductInfo{
		Id:        product.Id,
		Images:    []string{getImageByLanguage(req.Language, product.Cover)},
		ProjectId: product.ProjectId,
	}

	if res.KeyProduct.Name, err = product.GetLocalizedName(req.Language); err != nil {
		res.KeyProduct.Name, _ = product.GetLocalizedName(DefaultLanguage)
	}

	if res.KeyProduct.Description, err = product.GetLocalizedDescription(req.Language); err != nil {
		res.KeyProduct.Description, _ = product.GetLocalizedDescription(DefaultLanguage)
	}

	if res.KeyProduct.LongDescription, err = product.GetLocalizedLongDescription(req.Language); err != nil {
		res.KeyProduct.LongDescription, _ = product.GetLocalizedLongDescription(DefaultLanguage)
	}

	defaultPriceGroup, err := s.priceGroupRepository.GetByRegion(ctx, product.DefaultCurrency)
	if err != nil {
		zap.S().Errorw("Failed to get price group for default currency", "currency", product.DefaultCurrency)
		return keyProductInternalError
	}

	priceGroup := &billingpb.PriceGroup{}
	globalIsFallback := false
	if req.Currency != "" {
		priceGroup, err = s.priceGroupRepository.GetByRegion(ctx, req.Currency)
		if err != nil {
			zap.S().Errorw("Failed to get price group for specified currency", "currency", req.Currency)
			priceGroup = defaultPriceGroup
			globalIsFallback = true
		}
	} else {
		if req.Country != "" {
			err = s.GetPriceGroupByCountry(ctx, &billingpb.PriceGroupByCountryRequest{Country: req.Country}, priceGroup)
			if err != nil {
				zap.S().Error("could not get price group by country", "country", req.Country)
				priceGroup = defaultPriceGroup
				globalIsFallback = true
			}
		}
	}

	platforms := make([]*billingpb.PlatformPriceInfo, len(product.Platforms))
	for i, p := range product.Platforms {
		currency := priceGroup.Currency
		region := priceGroup.Region
		amount, err := product.GetPriceInCurrencyAndPlatform(priceGroup, p.Id)
		isFallback := globalIsFallback
		if err != nil {
			zap.S().Error("could not get price in currency and platform", "price_group", priceGroup, "platform", p.Id)
			isFallback = true
			currency = defaultPriceGroup.Currency
			region = defaultPriceGroup.Region
			amount, err = product.GetPriceInCurrencyAndPlatform(defaultPriceGroup, p.Id)
			if err != nil {
				zap.S().Error("could not get price in currency and platform for default price group", "price_group", defaultPriceGroup, "platform", p.Id)
				res.Status = billingpb.ResponseStatusSystemError
				res.Message = keyProductInternalError
				return nil
			}
		}

		platforms[i] = &billingpb.PlatformPriceInfo{
			Name: p.Name,
			Id:   p.Id,
			Price: &billingpb.ProductPriceInfo{
				Amount:     amount,
				Currency:   currency,
				Region:     region,
				IsFallback: isFallback,
			},
		}
	}

	sort.Slice(platforms, func(i, j int) bool {
		platform1 := &billingpb.Platform{}
		platform2 := &billingpb.Platform{}
		ok := false
		if platform1, ok = availablePlatforms[platforms[i].Id]; !ok {
			return false
		}
		if platform2, ok = availablePlatforms[platforms[i].Id]; !ok {
			return false
		}
		return platform1.Order < platform2.Order
	})

	res.KeyProduct.Platforms = platforms

	return nil
}

func (s *Service) GetKeyProduct(
	ctx context.Context,
	req *billingpb.RequestKeyProductMerchant,
	res *billingpb.KeyProductResponse,
) error {
	res.Status = billingpb.ResponseStatusOk
	product, err := s.keyProductRepository.GetById(ctx, req.Id)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = keyProductNotFound
			return nil
		}

		zap.S().Errorf("Query to find key product by id failed", "err", err.Error(), "data", req)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductRetrieveError
		return nil
	}

	if product.MerchantId != req.MerchantId {
		zap.S().Errorw("Merchant for product is mismatch with requested", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductMerchantMismatch
		return nil
	}

	res.Product = product

	return nil
}

func (s *Service) DeleteKeyProduct(
	ctx context.Context,
	req *billingpb.RequestKeyProductMerchant,
	res *billingpb.EmptyResponseWithStatus,
) error {
	product, err := s.keyProductRepository.GetById(ctx, req.Id)
	res.Status = billingpb.ResponseStatusOk

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = keyProductNotFound
			return nil
		}

		zap.S().Errorf("Error during getting key product", "err", err.Error(), "data", req)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductRetrieveError
		return nil
	}

	if product.MerchantId != req.MerchantId {
		zap.S().Errorw("Merchant for product is mismatch with requested", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductMerchantMismatch
		return nil
	}

	product.Deleted = true
	product.UpdatedAt = ptypes.TimestampNow()

	if err = s.keyProductRepository.Update(ctx, product); err != nil {
		res.Status = http.StatusInternalServerError
		res.Message = keyProductErrorDelete
		return nil
	}

	return nil
}

func (s *Service) PublishKeyProduct(
	ctx context.Context,
	req *billingpb.PublishKeyProductRequest,
	res *billingpb.KeyProductResponse,
) error {
	product, err := s.keyProductRepository.GetById(ctx, req.KeyProductId)
	res.Status = billingpb.ResponseStatusOk

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = keyProductNotFound
			return nil
		}

		zap.S().Errorf("Error during getting key product", "err", err.Error(), "data", req)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductRetrieveError
		return nil
	}

	if product.MerchantId != req.MerchantId {
		zap.S().Errorw("Merchant for product is mismatch with requested", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductMerchantMismatch
		return nil
	}

	product.UpdatedAt = ptypes.TimestampNow()
	product.PublishedAt = ptypes.TimestampNow()
	product.Enabled = true

	if err = s.keyProductRepository.Update(ctx, product); err != nil {
		res.Status = http.StatusInternalServerError
		res.Message = keyProductErrorUpsert
		return nil
	}

	res.Product = product
	return nil
}

func (s *Service) GetKeyProductsForOrder(
	ctx context.Context,
	req *billingpb.GetKeyProductsForOrderRequest,
	res *billingpb.ListKeyProductsResponse,
) error {
	if len(req.Ids) == 0 {
		zap.S().Errorf("Ids list is empty", "data", req)
		res.Status = http.StatusBadRequest
		res.Message = keyProductIdsIsEmpty
		return nil
	}

	list, err := s.keyProductRepository.FindByIdsProjectId(ctx, req.Ids, req.ProjectId)

	if err != nil {
		res.Status = http.StatusInternalServerError
		res.Message = keyProductInternalError.GetResponseErrorWithDetails(err.Error())
		return nil
	}

	res.Limit = int64(len(list))
	res.Offset = 0
	res.Count = res.Limit
	res.Products = list
	return nil
}

func (s *Service) ChangeCodeInOrder(ctx context.Context, req *billingpb.ChangeCodeInOrderRequest, res *billingpb.ChangeCodeInOrderResponse) error {
	res.Status = billingpb.ResponseStatusOk

	order, err := s.getOrderByUuid(ctx, req.OrderId)
	if err != nil {
		zap.S().Error("Query to get order failed", "err", err.Error(), "data", req)
		if messageErr, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = messageErr
			return nil
		}
		res.Status = http.StatusInternalServerError
		res.Message = keyProductInternalError
		res.Message.Details = err.Error()
		return nil
	}

	if order.GetPublicStatus() != recurringpb.OrderPublicStatusProcessed {
		zap.S().Error("Trying to change order what has not been processed.", "status", order.GetPublicStatus(), "data", req)
		res.Status = billingpb.ResponseStatusBadData
		res.Message = keyProductOrderIsNotProcessedError
		return nil
	}

	rsp := &billingpb.PlatformKeyReserveResponse{}
	err = s.ReserveKeyForOrder(ctx, &billingpb.PlatformKeyReserveRequest{
		OrderId:      order.Id,
		KeyProductId: req.KeyProductId,
		PlatformId:   order.PlatformId,
		MerchantId:   order.GetMerchantId(),
		Ttl:          oneDayTtl, // one day
	}, rsp)

	if err != nil {
		zap.S().Error("Reserving key for order is failed", "err", err.Error(), "data", req)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductInternalError
		res.Message.Details = err.Error()
		return nil
	}

	if rsp.Status != billingpb.ResponseStatusOk {
		zap.S().Error("Reserving key for order is failed", "data", req)
		res.Status = rsp.Status
		res.Message = rsp.Message
		return nil
	}

	keyRsp := &billingpb.GetKeyForOrderRequestResponse{}
	keyReq := &billingpb.KeyForOrderRequest{KeyId: rsp.KeyId}
	err = s.FinishRedeemKeyForOrder(ctx, keyReq, keyRsp)
	if err != nil {
		zap.S().Error("Finishing reserving key for order is failed", "err", err.Error(), "data", keyReq)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductInternalError
		res.Message.Details = err.Error()

		cancelRsp := &billingpb.EmptyResponseWithStatus{}
		err = s.CancelRedeemKeyForOrder(ctx, keyReq, cancelRsp)
		if err != nil {
			zap.S().Error("Cancelling reserving key for order is failed", "err", err.Error(), "data", keyReq)
		}

		return nil
	}

	if keyRsp.Status != billingpb.ResponseStatusOk {
		zap.S().Error("Can't finish redeeming key for order", "response", keyRsp, "data", keyReq)
		res.Status = keyRsp.Status
		res.Message = keyRsp.Message
		return nil
	}

	s.sendMailWithCode(ctx, order, keyRsp.Key)
	order.PrivateStatus = recurringpb.OrderStatusItemReplaced

	err = s.updateOrder(ctx, order)
	if err != nil {
		zap.S().Error("Error during updating order", "err", err.Error(), "data", req)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductInternalError
		res.Message.Details = err.Error()
		return nil
	}

	s.orderNotifyMerchant(ctx, order)

	res.Order = order
	return nil
}

func (s *Service) UnPublishKeyProduct(
	ctx context.Context,
	req *billingpb.UnPublishKeyProductRequest,
	res *billingpb.KeyProductResponse,
) error {
	product, err := s.keyProductRepository.GetById(ctx, req.KeyProductId)
	res.Status = billingpb.ResponseStatusOk

	if err != nil {
		if err == mongo.ErrNoDocuments {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = keyProductNotFound
			return nil
		}

		zap.S().Errorw("Error during getting key product", "err", err.Error(), "data", req)
		res.Status = http.StatusInternalServerError
		res.Message = keyProductRetrieveError
		return nil
	}

	if product.MerchantId != req.MerchantId {
		zap.S().Errorw("Key product not published", "key_product", req.KeyProductId)
		res.Status = http.StatusBadRequest
		res.Message = keyProductMerchantMismatch
		return nil
	}

	if product.Enabled == false {
		zap.S().Errorw("Key product not published", "key_product", req.KeyProductId)
		res.Status = http.StatusBadRequest
		res.Message = keyProductNotPublished
		return nil
	}

	product.Enabled = false

	if err = s.keyProductRepository.Update(ctx, product); err != nil {
		res.Status = http.StatusInternalServerError
		res.Message = keyProductErrorUpsert
		return nil
	}

	res.Product = product

	return nil
}

func getImageByLanguage(lng string, collection *billingpb.ImageCollection) string {
	if collection == nil || collection.Images == nil {
		return ""
	}

	lng = strings.ToLower(lng)
	var image = ""

	switch lng {
	case "en":
		image = collection.Images.En
	case "ru":
		image = collection.Images.Ru
	case "fr":
		image = collection.Images.Fr
	case "es":
		image = collection.Images.Es
	case "de":
		image = collection.Images.De
	case "zh":
		image = collection.Images.Zh
	case "ar":
		image = collection.Images.Ar
	case "pt":
		image = collection.Images.Pt
	case "it":
		image = collection.Images.It
	case "pl":
		image = collection.Images.Pl
	case "tr":
		image = collection.Images.Tr
	case "el":
		image = collection.Images.El
	case "ko":
		image = collection.Images.Ko
	case "vl":
		image = collection.Images.Vl
	case "ja":
		image = collection.Images.Ja
	case "he":
		image = collection.Images.He
	case "th":
		image = collection.Images.Th
	case "cs":
		image = collection.Images.Cs
	case "bg":
		image = collection.Images.Bg
	case "fi":
		image = collection.Images.Fi
	case "sv":
		image = collection.Images.Sv
	case "da":
		image = collection.Images.Da
	}

	if image == "" {
		image = collection.Images.En
	}

	return image
}
