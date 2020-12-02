package repository

import (
	"context"
	"time"
)

// DashboardProcessorRepositoryInterface is abstraction layer for working with dashboard report and representation in database.
type DashboardProcessorRepositoryInterface interface {
	ExecuteReport(ctx context.Context, out interface{}, fn func(context.Context, interface{}) (interface{}, error)) (interface{}, error)
	ExecuteGrossRevenueAndVatReports(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteTotalTransactionsAndArpuReports(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteRevenueDynamicReport(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteRevenueByCountryReport(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteSalesTodayReport(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteSourcesReport(ctx context.Context, out interface{}) (interface{}, error)

	ExecuteCustomerLTV(ctx context.Context, out interface{}) (interface{}, interface{}, error)
	ExecuteCustomerARPU(ctx context.Context, customerId string) (interface{}, error)
	ExecuteCustomerAvgTransactionsCount(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteCustomerTop20(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteCustomers(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteCustomersCount(ctx context.Context, out interface{}) (interface{}, error)
	ExecuteCustomersChart(ctx context.Context, startDate time.Time, end time.Time) (interface{}, error)
	ExecuteCustomerARPPU(ctx context.Context) (interface{}, error)
}
