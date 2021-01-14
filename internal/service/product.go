package service

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

var (
	productErrorUnknown                    = errors.NewBillingServerErrorMsg("pd000001", "unknown error with product")
	productErrorNotFound                   = errors.NewBillingServerErrorMsg("pd000002", "products with specified ID not found")
	productErrorIdListEmpty                = errors.NewBillingServerErrorMsg("pd000007", "ids list is empty")
	productErrorMerchantNotEqual           = errors.NewBillingServerErrorMsg("pd000008", "merchant id is not equal in a product")
	productErrorProjectNotEqual            = errors.NewBillingServerErrorMsg("pd000009", "project id is not equal in a product")
	productErrorPriceDefaultCurrency       = errors.NewBillingServerErrorMsg("pd000010", "no price in default currency")
	productErrorNameDefaultLanguage        = errors.NewBillingServerErrorMsg("pd000011", "no name in default language")
	productErrorDescriptionDefaultLanguage = errors.NewBillingServerErrorMsg("pd000012", "no description in default language")
	productErrorUpsert                     = errors.NewBillingServerErrorMsg("pd000013", "query to insert/update product failed")
	productErrorDelete                     = errors.NewBillingServerErrorMsg("pd000014", "query to delete product failed")
	productErrorProjectAndSkuAlreadyExists = errors.NewBillingServerErrorMsg("pd000015", "pair projectId+Sku already exists")
	productErrorListPrices                 = errors.NewBillingServerErrorMsg("pd000016", "list of prices is empty")
	productErrorPricesUpdate               = errors.NewBillingServerErrorMsg("pd000017", "query to update product prices is failed")
	productSkuMismatch                     = errors.NewBillingServerErrorMsg("pd000018", "sku mismatch")
	productNoPriceInCurrencyError          = errors.NewBillingServerErrorMsg("pd000019", "no product price in requested currency")
)

func (s *Service) CreateOrUpdateProduct(ctx context.Context, req *billingpb.Product, res *billingpb.Product) error {
	var (
		err     error
		product = &billingpb.Product{}
		isNew   = req.Id == ""
		now     = ptypes.TimestampNow()
	)

	if isNew {
		req.Id = primitive.NewObjectID().Hex()
		req.CreatedAt = now
	} else {
		product, err = s.productRepository.GetById(ctx, req.Id)
		if err != nil {
			zap.S().Errorf("Product that requested to change is not found", "err", err.Error(), "data", req)
			return productErrorNotFound
		}

		if req.Sku != "" && req.Sku != product.Sku {
			zap.S().Errorf("SKU mismatch", "data", req)
			return productSkuMismatch
		}

		if req.MerchantId != product.MerchantId {
			zap.S().Errorf("MerchantId mismatch", "data", req)
			return productErrorMerchantNotEqual
		}

		if req.ProjectId != product.ProjectId {
			zap.S().Errorf("ProjectId mismatch", "data", req)
			return productErrorProjectNotEqual
		}

		req.CreatedAt = product.CreatedAt
	}
	req.UpdatedAt = now
	req.Deleted = false

	if !req.IsPricesContainDefaultCurrency() {
		zap.S().Errorf(productErrorPriceDefaultCurrency.Message, "data", req)
		return productErrorPriceDefaultCurrency
	}

	if _, err := req.GetLocalizedName(DefaultLanguage); err != nil {
		zap.S().Errorf("No name in default language", "data", req)
		return productErrorNameDefaultLanguage
	}

	if _, err := req.GetLocalizedDescription(DefaultLanguage); err != nil {
		zap.S().Errorf("No description in default language", "data", req)
		return productErrorDescriptionDefaultLanguage
	}

	count, err := s.productRepository.CountByProjectSku(ctx, req.ProjectId, req.Sku)

	if err != nil {
		zap.S().Errorf("Query to find duplicates failed", "err", err.Error(), "data", req)
		return productErrorUnknown
	}

	allowed := int64(1)

	if isNew {
		allowed = 0
	}

	if count > allowed {
		zap.S().Errorf("Pair projectId+Sku already exists", "data", req)
		return productErrorProjectAndSkuAlreadyExists
	}

	if err = s.productRepository.Upsert(ctx, req); err != nil {
		zap.S().Errorf("Query to create/update product failed", "err", err.Error(), "data", req)
		return productErrorUpsert
	}

	res.Id = req.Id
	res.Object = req.Object
	res.Type = req.Type
	res.Sku = req.Sku
	res.Name = req.Name
	res.DefaultCurrency = req.DefaultCurrency
	res.Enabled = req.Enabled
	res.Prices = req.Prices
	res.Description = req.Description
	res.LongDescription = req.LongDescription
	res.Images = req.Images
	res.Url = req.Url
	res.Metadata = req.Metadata
	res.CreatedAt = req.CreatedAt
	res.UpdatedAt = req.UpdatedAt
	res.Deleted = req.Deleted
	res.MerchantId = req.MerchantId
	res.ProjectId = req.ProjectId
	res.Pricing = req.Pricing
	res.BillingType = req.BillingType

	return nil
}

func (s *Service) GetProductsForOrder(ctx context.Context, req *billingpb.GetProductsForOrderRequest, res *billingpb.ListProductsResponse) error {
	if len(req.Ids) == 0 {
		zap.S().Errorf("Ids list is empty", "data", req)
		return productErrorIdListEmpty
	}

	var found []*billingpb.Product

	for _, id := range req.Ids {
		p, err := s.productRepository.GetById(ctx, id)

		if err != nil {
			zap.S().Errorf("Unable to get product", "err", err.Error(), "req", req)
			continue
		}

		if p.Enabled != true || p.ProjectId != req.ProjectId {
			continue
		}

		found = append(found, p)
	}

	res.Limit = int64(len(found))
	res.Offset = 0
	res.Total = res.Limit
	res.Products = found
	return nil
}

func (s *Service) ListProducts(ctx context.Context, req *billingpb.ListProductsRequest, res *billingpb.ListProductsResponse) error {
	var (
		enabled int32
		err     error
	)

	switch req.Enabled {
	case "false":
		enabled = 1
		break
	case "true":
		enabled = 2
		break
	default:
		enabled = 0
	}

	res.Total, err = s.productRepository.FindCount(ctx, req.MerchantId, req.ProjectId, req.Sku, req.Name, enabled)

	if err != nil {
		return nil
	}

	if res.Total == 0 || req.Offset > res.Total {
		zap.L().Error(
			"total is empty or less then total",
			zap.Error(err),
			zap.Int64(pkg.ErrorDatabaseFieldLimit, req.Limit),
			zap.Int64(pkg.ErrorDatabaseFieldOffset, req.Offset),
		)
		return nil
	}

	res.Products, err = s.productRepository.Find(
		ctx,
		req.MerchantId,
		req.ProjectId,
		req.Sku,
		req.Name,
		enabled,
		req.Offset,
		req.Limit,
	)

	if err != nil {
		return nil
	}

	res.Limit = req.Limit
	res.Offset = req.Offset

	return nil
}

func (s *Service) GetProduct(
	ctx context.Context,
	req *billingpb.RequestProduct,
	rsp *billingpb.GetProductResponse,
) error {
	product, err := s.productRepository.GetById(ctx, req.Id)

	if err != nil {
		zap.L().Error(
			"Unable to get product",
			zap.Error(err),
			zap.Any(pkg.LogFieldRequest, req),
		)

		rsp.Status = billingpb.ResponseStatusNotFound
		rsp.Message = productErrorNotFound

		return nil
	}

	if req.MerchantId != product.MerchantId {
		zap.L().Error(
			"Merchant id mismatch",
			zap.Any("product", product),
			zap.Any(pkg.LogFieldRequest, req),
		)

		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = productErrorMerchantNotEqual

		return nil
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Item = product

	return nil
}

func (s *Service) DeleteProduct(ctx context.Context, req *billingpb.RequestProduct, res *billingpb.EmptyResponse) error {
	product, err := s.productRepository.GetById(ctx, req.Id)

	if err != nil {
		zap.S().Errorf("Unable to get product", "err", err.Error(), "req", req)
		return productErrorNotFound
	}

	if req.MerchantId != product.MerchantId {
		zap.S().Errorf("MerchantId mismatch", "product", product, "data", req)
		return productErrorMerchantNotEqual
	}

	product.Deleted = true
	product.UpdatedAt = ptypes.TimestampNow()

	err = s.productRepository.Upsert(ctx, product)

	if err != nil {
		zap.S().Errorf("Query to delete product failed", "err", err.Error(), "data", req)
		return productErrorDelete
	}

	return nil
}

func (s *Service) GetProductPrices(ctx context.Context, req *billingpb.RequestProduct, res *billingpb.ProductPricesResponse) error {
	product, err := s.productRepository.GetById(ctx, req.Id)

	if err != nil {
		zap.S().Errorf("Unable to get product", "err", err.Error(), "req", req)
		return productErrorNotFound
	}

	if req.MerchantId != product.MerchantId {
		zap.S().Errorf("MerchantId mismatch", "product", product, "data", req)
		return productErrorMerchantNotEqual
	}

	res.ProductPrice = product.Prices

	return nil
}

func (s *Service) UpdateProductPrices(ctx context.Context, req *billingpb.UpdateProductPricesRequest, res *billingpb.ResponseError) error {
	if len(req.Prices) == 0 {
		zap.S().Errorf("List of product prices is empty", "data", req)
		return productErrorListPrices
	}

	product, err := s.productRepository.GetById(ctx, req.ProductId)

	if err != nil {
		zap.S().Errorf("Unable to get product", "err", err.Error(), "req", req)
		return productErrorNotFound
	}

	if req.MerchantId != product.MerchantId {
		zap.S().Errorf("MerchantId mismatch", "product", product, "data", req)
		return productErrorMerchantNotEqual
	}

	product.Prices = req.Prices

	// note: virtual currency has IsVirtualCurrency=true && Currency=""
	for _, p := range product.Prices {
		if p.IsVirtualCurrency == true {
			p.Currency = ""
		}
	}

	merchant, err := s.merchantRepository.GetById(ctx, product.MerchantId)
	if err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = merchantErrorNotFound

		return nil
	}

	payoutCurrency := merchant.GetPayoutCurrency()

	if len(payoutCurrency) == 0 {
		zap.S().Errorw(merchantPayoutCurrencyMissed.Message, "data", req)
		res.Status = billingpb.ResponseStatusBadData
		res.Message = merchantPayoutCurrencyMissed
		return nil
	}

	_, err = product.GetPriceInCurrency(&billingpb.PriceGroup{Currency: payoutCurrency})
	if err != nil {
		_, err = product.GetPriceInCurrency(&billingpb.PriceGroup{Currency: billingpb.VirtualCurrencyPriceGroup})
	}

	if err != nil {
		zap.S().Errorw(productErrorPriceDefaultCurrency.Message, "data", req)
		return productErrorPriceDefaultCurrency
	}

	if !product.IsPricesContainDefaultCurrency() {
		zap.S().Errorf(productErrorPriceDefaultCurrency.Message, "data", req)
		return productErrorPriceDefaultCurrency
	}

	if err := s.productRepository.Upsert(ctx, product); err != nil {
		zap.S().Errorf("Query to create/update product failed", "err", err.Error(), "data", req)
		return productErrorPricesUpdate
	}

	return nil
}
