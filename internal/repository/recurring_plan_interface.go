package repository

import (
	"context"
	"github.com/paysuper/paysuper-proto/go/billingpb"
)

// RecurringPlanRepositoryInterface is abstraction layer for working with recurring plan information and representation in database.
type RecurringPlanRepositoryInterface interface {
	// Insert adds recurring plan to the collection.
	Insert(context.Context, *billingpb.RecurringPlan) error

	// Update updates the recurring plan in the collection.
	Update(context.Context, *billingpb.RecurringPlan) error

	// GetById returns the recurring plan by unique identity.
	GetById(context.Context, string) (*billingpb.RecurringPlan, error)

	// Find recurring plans by merchant, project, external id, group id and query string with pagination.
	Find(ctx context.Context, merchantId, projectId, externalId, groupId, query string, offset, count int32) ([]*billingpb.RecurringPlan, error)

	// FindCount return count of recurring plans by merchant, project, external id, group id and query.
	FindCount(ctx context.Context, merchantId, projectId, externalId, groupId, query string) (int64, error)
}
