package repository

import (
	"context"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OrderRepositoryInterface is abstraction layer for working with order and representation in database.
type OrderRepositoryInterface interface {
	// Insert adds order to the collection.
	Insert(context.Context, *billingpb.Order) error

	// Update updates the order in the collection.
	Update(context.Context, *billingpb.Order) error

	// GetById returns a order by its identifier.
	GetById(context.Context, string) (*billingpb.Order, error)

	// GetByUuid returns a order by its public (uuid) identifier.
	GetByUuid(context.Context, string) (*billingpb.Order, error)

	// GetByUuid returns a order by its public (uuid) identifier and merchant identifier.
	GetByUuidAndMerchantId(ctx context.Context, uuid string, merchantId string) (*billingpb.Order, error)

	// GetByRefundReceiptNumber returns a order by its receipt number.
	GetByRefundReceiptNumber(context.Context, string) (*billingpb.Order, error)

	// UpdateOrderView updates orders into order view.
	UpdateOrderView(context.Context, []string) error

	// Return order by some conditions
	GetOneBy(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) (*billingpb.Order, error)

	// Mark orders as included to royalty report.
	IncludeOrdersToRoyaltyReport(ctx context.Context, royaltyReportId string, orderIds []primitive.ObjectID) error

	// Get first payment for merchant
	GetFirstPaymentForMerchant(ctx context.Context, merchantId string) (*billingpb.Order, error)

	// Return orders by some conditions and with options
	GetManyBy(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]*billingpb.Order, error)

	// GetBySubscriptionId returns orders by subscription identifier.
	GetBySubscriptionId(ctx context.Context, subscriptionId string) (items []*billingpb.Order, err error)
}
