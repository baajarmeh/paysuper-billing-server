package main

import (
	_ "github.com/micro/go-plugins/registry/kubernetes"
	"github.com/paysuper/paysuper-billing-server/internal"
	"go.uber.org/zap"
	"strings"
)

func main() {
	app := internal.NewApplication()
	app.Init()

	task := app.CliArgs.Get("task").String("")
	date := app.CliArgs.Get("date").String("")
	orderId := app.CliArgs.Get("orderid").String("")
	force := strings.ToLower(app.CliArgs.Get("force").String("")) == "true"

	if task != "" {

		defer app.Stop()

		var err error

		switch task {
		case "vat_reports":
			err = app.TaskProcessVatReports(date)

		case "royalty_reports":
			err = app.TaskCreateRoyaltyReport()

		case "royalty_reports_accept":
			err = app.TaskAutoAcceptRoyaltyReports()

		case "create_payouts":
			err = app.TaskAutoCreatePayouts()

		case "rebuild_order_view":
			err = app.TaskRebuildOrderView()

		case "merchants_migrate":
			err = app.TaskMerchantsMigrate()

		case "fix_taxes":
			err = app.TaskFixTaxes()

		case "migrate_customers":
			err = app.MigrateCustomers()
			break

		case "update_merchants_first_payment":
			err = app.UpdateFirstPayments()
			break

		case "rebuild_accounting_entries":
			err = app.TaskRebuildAccountingEntries(orderId, force)
			break

		case "fix_reports_dates":
			err = app.TaskFixReportDates()
			break

		case "extend_payouts_with_vat":
			err = app.TaskExtendPayoutsWithVat()
			break
		}

		if err != nil {
			zap.L().Fatal("task error",
				zap.Error(err),
				zap.String("task", task),
				zap.String("date", date),
			)
		}

		return
	}

	app.KeyDaemonStart()

	app.Run()
}
