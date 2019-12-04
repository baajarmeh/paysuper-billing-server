// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import billing "github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
import context "context"
import mock "github.com/stretchr/testify/mock"
import pkg "github.com/paysuper/paysuper-billing-server/internal/pkg"

// PaymentChannelCostMerchantInterface is an autogenerated mock type for the PaymentChannelCostMerchantInterface type
type PaymentChannelCostMerchantInterface struct {
	mock.Mock
}

// Delete provides a mock function with given fields: ctx, obj
func (_m *PaymentChannelCostMerchantInterface) Delete(ctx context.Context, obj *billing.PaymentChannelCostMerchant) error {
	ret := _m.Called(ctx, obj)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billing.PaymentChannelCostMerchant) error); ok {
		r0 = rf(ctx, obj)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: ctx, merchantId, name, payoutCurrency, region, country, mccCode
func (_m *PaymentChannelCostMerchantInterface) Get(ctx context.Context, merchantId string, name string, payoutCurrency string, region string, country string, mccCode string) ([]*pkg.PaymentChannelCostMerchantSet, error) {
	ret := _m.Called(ctx, merchantId, name, payoutCurrency, region, country, mccCode)

	var r0 []*pkg.PaymentChannelCostMerchantSet
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string, string, string) []*pkg.PaymentChannelCostMerchantSet); ok {
		r0 = rf(ctx, merchantId, name, payoutCurrency, region, country, mccCode)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*pkg.PaymentChannelCostMerchantSet)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string, string, string) error); ok {
		r1 = rf(ctx, merchantId, name, payoutCurrency, region, country, mccCode)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAllForMerchant provides a mock function with given fields: ctx, merchantId
func (_m *PaymentChannelCostMerchantInterface) GetAllForMerchant(ctx context.Context, merchantId string) (*billing.PaymentChannelCostMerchantList, error) {
	ret := _m.Called(ctx, merchantId)

	var r0 *billing.PaymentChannelCostMerchantList
	if rf, ok := ret.Get(0).(func(context.Context, string) *billing.PaymentChannelCostMerchantList); ok {
		r0 = rf(ctx, merchantId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billing.PaymentChannelCostMerchantList)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, merchantId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetById provides a mock function with given fields: ctx, id
func (_m *PaymentChannelCostMerchantInterface) GetById(ctx context.Context, id string) (*billing.PaymentChannelCostMerchant, error) {
	ret := _m.Called(ctx, id)

	var r0 *billing.PaymentChannelCostMerchant
	if rf, ok := ret.Get(0).(func(context.Context, string) *billing.PaymentChannelCostMerchant); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billing.PaymentChannelCostMerchant)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MultipleInsert provides a mock function with given fields: ctx, obj
func (_m *PaymentChannelCostMerchantInterface) MultipleInsert(ctx context.Context, obj []*billing.PaymentChannelCostMerchant) error {
	ret := _m.Called(ctx, obj)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []*billing.PaymentChannelCostMerchant) error); ok {
		r0 = rf(ctx, obj)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Update provides a mock function with given fields: ctx, obj
func (_m *PaymentChannelCostMerchantInterface) Update(ctx context.Context, obj *billing.PaymentChannelCostMerchant) error {
	ret := _m.Called(ctx, obj)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billing.PaymentChannelCostMerchant) error); ok {
		r0 = rf(ctx, obj)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}