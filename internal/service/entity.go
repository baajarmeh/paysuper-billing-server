package service

import (
	"github.com/paysuper/paysuper-billing-server/pkg"
	"go.uber.org/zap"
	"net"
	"net/url"
	"sync"
	"time"
)

type Entity struct {
	svc *Service
	mx  sync.Mutex
}

type OrderView Entity
type Accounting Entity

type Repository struct {
	svc *Service
}

type MerchantsTariffRatesRepository Repository
type DashboardRepository Repository

type kvIntFloat struct {
	Key   int
	Value float64
}

type kvIntInt struct {
	Key   int
	Value int32
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	zap.L().Info(
		"function execution time",
		zap.String("name", name),
		zap.Duration("time", elapsed),
	)
}

func getHostFromUrl(urlString string) string {
	u, err := url.Parse(urlString)
	if err != nil {
		zap.L().Error(
			"url parsing failed",
			zap.Error(err),
		)
		return ""
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return u.Host
	}
	return host
}

func getMccByOperationsType(operationsType string) (string, error) {
	mccCode, ok := pkg.MerchantOperationsTypesToMccCodes[operationsType]
	if !ok {
		return "", merchantErrorOperationsTypeNotSupported
	}
	return mccCode, nil
}
