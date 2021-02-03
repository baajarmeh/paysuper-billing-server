package repository

import (
	"context"
	internalPkg "github.com/paysuper/paysuper-billing-server/internal/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
)

// PaymentChannelCostMerchantRepositoryInterface is abstraction layer for working with payment channel cost
// for merchant and representation in database.
type PaymentChannelCostMerchantRepositoryInterface interface {
	// Insert adds the payment channel cost to the collection.
	Insert(context.Context, *billingpb.PaymentChannelCostMerchant) error

	// Insert adds the multiple payment channel costs to the collection.
	MultipleInsert(context.Context, []*billingpb.PaymentChannelCostMerchant) error

	// Update updates the payment channel cost in the collection.
	Update(context.Context, *billingpb.PaymentChannelCostMerchant) error

	// Delete вудуеу the payment channel cost in the collection.
	Delete(ctx context.Context, obj *billingpb.PaymentChannelCostMerchant) error

	// GetById returns the payment channel cost by unique identity.
	GetById(context.Context, string) (*billingpb.PaymentChannelCostMerchant, error)

	// GetAllForMerchant returns all payment channel costs for merchant.
	GetAllForMerchant(context.Context, string) ([]*billingpb.PaymentChannelCostMerchant, error)

	// GetActiveForMerchant returns active payment channel costs for merchant.
	GetActiveForMerchant(context.Context, string) ([]*billingpb.PaymentChannelCostMerchant, error)

	// Find returns the payment channel costs by merchant id, name, payout currency, region, country and mcc code.
	Find(context.Context, string, string, string, string, string, string) ([]*internalPkg.PaymentChannelCostMerchantSet, error)

	// Update all merchant's tariffs
	DeleteAndInsertMany(ctx context.Context, merchantId string, tariffs []*billingpb.PaymentChannelCostMerchant) error
}
