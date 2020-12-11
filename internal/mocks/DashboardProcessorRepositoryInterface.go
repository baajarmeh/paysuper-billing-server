// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	time "time"
)

// DashboardProcessorRepositoryInterface is an autogenerated mock type for the DashboardProcessorRepositoryInterface type
type DashboardProcessorRepositoryInterface struct {
	mock.Mock
}

// ExecuteCustomerARPPU provides a mock function with given fields: ctx
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomerARPPU(ctx context.Context) (interface{}, error) {
	ret := _m.Called(ctx)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context) interface{}); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteCustomerARPU provides a mock function with given fields: ctx, customerId
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomerARPU(ctx context.Context, customerId string) (interface{}, error) {
	ret := _m.Called(ctx, customerId)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, string) interface{}); ok {
		r0 = rf(ctx, customerId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, customerId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteCustomerAvgTransactionsCount provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomerAvgTransactionsCount(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteCustomerLTV provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomerLTV(ctx context.Context, out interface{}) (interface{}, interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 interface{}
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) interface{}); ok {
		r1 = rf(ctx, out)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(interface{})
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, interface{}) error); ok {
		r2 = rf(ctx, out)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ExecuteCustomerTop20 provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomerTop20(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteCustomers provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomers(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteCustomersChart provides a mock function with given fields: ctx, startDate, end
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomersChart(ctx context.Context, startDate time.Time, end time.Time) (interface{}, error) {
	ret := _m.Called(ctx, startDate, end)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, time.Time) interface{}); ok {
		r0 = rf(ctx, startDate, end)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, time.Time, time.Time) error); ok {
		r1 = rf(ctx, startDate, end)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteCustomersCount provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteCustomersCount(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteGrossRevenueAndVatReports provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteGrossRevenueAndVatReports(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteReport provides a mock function with given fields: ctx, out, fn
func (_m *DashboardProcessorRepositoryInterface) ExecuteReport(ctx context.Context, out interface{}, fn func(context.Context, interface{}) (interface{}, error)) (interface{}, error) {
	ret := _m.Called(ctx, out, fn)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}, func(context.Context, interface{}) (interface{}, error)) interface{}); ok {
		r0 = rf(ctx, out, fn)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}, func(context.Context, interface{}) (interface{}, error)) error); ok {
		r1 = rf(ctx, out, fn)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteRevenueByCountryReport provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteRevenueByCountryReport(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteRevenueDynamicReport provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteRevenueDynamicReport(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteSalesTodayReport provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteSalesTodayReport(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteSourcesReport provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteSourcesReport(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteTotalTransactionsAndArpuReports provides a mock function with given fields: ctx, out
func (_m *DashboardProcessorRepositoryInterface) ExecuteTotalTransactionsAndArpuReports(ctx context.Context, out interface{}) (interface{}, error) {
	ret := _m.Called(ctx, out)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) interface{}); ok {
		r0 = rf(ctx, out)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, interface{}) error); ok {
		r1 = rf(ctx, out)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
