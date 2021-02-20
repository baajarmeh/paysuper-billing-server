package repository

import (
	"context"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"time"
)

// RecurringSubscriptionRepositoryInterface is abstraction layer for working with recurring subscription information and representation in database.
type RecurringSubscriptionRepositoryInterface interface {
	// Insert adds recurring subscription to the collection.
	Insert(context.Context, *billingpb.RecurringSubscription) error

	// Update updates the recurring subscription in the collection.
	Update(context.Context, *billingpb.RecurringSubscription) error

	// GetById returns the recurring subscription by unique identity.
	GetById(context.Context, string) (*billingpb.RecurringSubscription, error)

	// GetByPlanIdCustomerId returns the recurring subscription by recurring plan and customer identity.
	GetByPlanIdCustomerId(ctx context.Context, planId, customerId string) (*billingpb.RecurringSubscription, error)

	// GetActiveByPlanIdCustomerId returns the active recurring subscription by recurring plan and customer identity.
	GetActiveByPlanIdCustomerId(ctx context.Context, planId, customerId string) (*billingpb.RecurringSubscription, error)

	// FindByCustomerId returns list of recurring subscriptions by customer identifier.
	FindByCustomerId(context.Context, string) ([]*billingpb.RecurringSubscription, error)

	// FindByMerchantIdCustomerId returns list of recurring subscriptions by merchant and customer identifier.
	FindByMerchantIdCustomerId(ctx context.Context, merchantId, customerId string) ([]*billingpb.RecurringSubscription, error)

	// FindExpired returns list of recurring subscriptions with expire time.
	FindExpired(ctx context.Context, expireAt time.Time) ([]*billingpb.RecurringSubscription, error)

	// FindByPlanId returns the recurring subscriptions by recurring plan identity.
	FindByPlanId(ctx context.Context, planId string) ([]*billingpb.RecurringSubscription, error)

	// Find returns subscription by params.
	Find(ctx context.Context, userId, merchantId, status, quickFilter string, dateFrom, dateTo *time.Time, limit, offset int64) (items []*billingpb.RecurringSubscription, err error)

	// FindCount returns count subscriptions by params.
	FindCount(ctx context.Context, userId, merchantId, status, quickFilter string, dateFrom, dateTo *time.Time) (count int64, err error)
}
