package repository

import (
	"context"
	"github.com/paysuper/paysuper-proto/go/billingpb"
)

// MerchantDocumentRepositoryInterface is abstraction layer for working with merchant documents in database.
type MerchantDocumentRepositoryInterface interface {
	// Insert adds the merchant document to the collection.
	Insert(context.Context, *billingpb.MerchantDocument) error

	// GetById get merchant document by identity
	GetById(context.Context, string) (*billingpb.MerchantDocument, error)

	// GetByMerchantId return documents for merchant
	GetByMerchantId(context.Context, string, int64, int64) ([]*billingpb.MerchantDocument, error)

	// CountByMerchantId return count documents for merchant
	CountByMerchantId(context.Context, string) (int64, error)
}
