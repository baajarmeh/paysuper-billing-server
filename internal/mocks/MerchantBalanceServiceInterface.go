// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import billing "github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
import context "context"
import mock "github.com/stretchr/testify/mock"

// MerchantBalanceServiceInterface is an autogenerated mock type for the MerchantBalanceServiceInterface type
type MerchantBalanceServiceInterface struct {
	mock.Mock
}

// GetByMerchantIdAndCurrency provides a mock function with given fields: ctx, merchantId, currency
func (_m *MerchantBalanceServiceInterface) GetByMerchantIdAndCurrency(ctx context.Context, merchantId string, currency string) (*billing.MerchantBalance, error) {
	ret := _m.Called(ctx, merchantId, currency)

	var r0 *billing.MerchantBalance
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *billing.MerchantBalance); ok {
		r0 = rf(ctx, merchantId, currency)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billing.MerchantBalance)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, merchantId, currency)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Insert provides a mock function with given fields: ctx, document
func (_m *MerchantBalanceServiceInterface) Insert(ctx context.Context, document *billing.MerchantBalance) error {
	ret := _m.Called(ctx, document)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billing.MerchantBalance) error); ok {
		r0 = rf(ctx, document)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
