package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type royaltyReportMapper struct{}

type royaltyReportCorrectionItemMapper struct{}

type royaltyReportChangesMapper struct{}

func NewRoyaltyReportMapper() Mapper {
	return &royaltyReportMapper{}
}

func NewRoyaltyReportCorrectionItemMapper() Mapper {
	return &royaltyReportCorrectionItemMapper{}
}

func NewRoyaltyReportChangesMapper() Mapper {
	return &royaltyReportChangesMapper{}
}

type MgoRoyaltyReport struct {
	Id                 primitive.ObjectID             `bson:"_id" faker:"objectId"`
	MerchantId         primitive.ObjectID             `bson:"merchant_id" faker:"objectId"`
	CreatedAt          time.Time                      `bson:"created_at"`
	UpdatedAt          time.Time                      `bson:"updated_at"`
	PayoutDate         time.Time                      `bson:"payout_date"`
	Status             string                         `bson:"status"`
	PeriodFrom         time.Time                      `bson:"period_from"`
	PeriodTo           time.Time                      `bson:"period_to"`
	AcceptExpireAt     time.Time                      `bson:"accept_expire_at"`
	AcceptedAt         time.Time                      `bson:"accepted_at"`
	Totals             *billingpb.RoyaltyReportTotals `bson:"totals"`
	Currency           string                         `bson:"currency"`
	Summary            *MgoRoyaltyReportSummary       `bson:"summary"`
	DisputeReason      string                         `bson:"dispute_reason"`
	DisputeStartedAt   time.Time                      `bson:"dispute_started_at"`
	DisputeClosedAt    time.Time                      `bson:"dispute_closed_at"`
	IsAutoAccepted     bool                           `bson:"is_auto_accepted"`
	PayoutDocumentId   string                         `bson:"payout_document_id"`
	OperatingCompanyId string                         `bson:"operating_company_id"`
	StringPeriodFrom   string                         `bson:"string_period_from"`
	StringPeriodTo     string                         `bson:"string_period_to"`
	Merchants          []*MgoMerchant                 `bson:"merchants" faker:"-"`
}

type MgoRoyaltyReportSummary struct {
	ProductsItems   []*billingpb.RoyaltyReportProductSummaryItem `json:"products_items" bson:"products_items"`
	ProductsTotal   *billingpb.RoyaltyReportProductSummaryItem   `json:"products_total" bson:"products_total"`
	Corrections     []*MgoRoyaltyReportCorrectionItem            `json:"corrections" bson:"corrections"`
	RollingReserves []*MgoRoyaltyReportCorrectionItem            `json:"rolling_reserves" bson:"rolling_reserves"`
}

type MgoRoyaltyReportChanges struct {
	Id              primitive.ObjectID `bson:"_id" faker:"objectId"`
	RoyaltyReportId primitive.ObjectID `bson:"royalty_report_id" faker:"objectId"`
	Source          string             `bson:"source"`
	Ip              string             `bson:"ip"`
	Hash            string             `bson:"hash"`
	CreatedAt       time.Time          `bson:"created_at"`
}

type MgoRoyaltyReportCorrectionItem struct {
	AccountingEntryId primitive.ObjectID `bson:"accounting_entry_id" faker:"objectId"`
	Amount            float64            `bson:"amount"`
	Currency          string             `bson:"currency"`
	Reason            string             `bson:"reason"`
	EntryDate         time.Time          `bson:"entry_date"`
}

func (m *royaltyReportMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.RoyaltyReport)

	out := &MgoRoyaltyReport{
		Status:             in.Status,
		Totals:             in.Totals,
		Currency:           in.Currency,
		DisputeReason:      in.DisputeReason,
		IsAutoAccepted:     in.IsAutoAccepted,
		PayoutDocumentId:   in.PayoutDocumentId,
		OperatingCompanyId: in.OperatingCompanyId,
		StringPeriodFrom:   in.StringPeriodFrom,
		StringPeriodTo:     in.StringPeriodTo,
	}

	if len(in.Id) <= 0 {
		out.Id = primitive.NewObjectID()
	} else {
		oid, err := primitive.ObjectIDFromHex(in.Id)
		if err != nil {
			return nil, err
		}
		out.Id = oid
	}

	if in.CreatedAt != nil {
		t, err := ptypes.Timestamp(in.CreatedAt)
		if err != nil {
			return nil, err
		}
		out.CreatedAt = t
	} else {
		out.CreatedAt = time.Now()
	}

	if in.UpdatedAt != nil {
		t, err := ptypes.Timestamp(in.UpdatedAt)
		if err != nil {
			return nil, err
		}
		out.UpdatedAt = t
	} else {
		out.UpdatedAt = time.Now()
	}

	merchantOid, err := primitive.ObjectIDFromHex(in.MerchantId)
	if err != nil {
		return nil, err
	}
	out.MerchantId = merchantOid

	t, err := ptypes.Timestamp(in.PeriodFrom)
	if err != nil {
		return nil, err
	}
	out.PeriodFrom = t

	t, err = ptypes.Timestamp(in.PeriodTo)
	if err != nil {
		return nil, err
	}
	out.PeriodTo = t

	t, err = ptypes.Timestamp(in.AcceptExpireAt)
	if err != nil {
		return nil, err
	}
	out.AcceptExpireAt = t

	if in.PayoutDate != nil {
		t, err := ptypes.Timestamp(in.PayoutDate)
		if err != nil {
			return nil, err
		}
		out.PayoutDate = t
	}

	if in.AcceptedAt != nil {
		t, err = ptypes.Timestamp(in.AcceptedAt)
		if err != nil {
			return nil, err
		}
		out.AcceptedAt = t
	}

	if in.DisputeStartedAt != nil {
		t, err := ptypes.Timestamp(in.DisputeStartedAt)
		if err != nil {
			return nil, err
		}
		out.DisputeStartedAt = t
	}

	if in.DisputeClosedAt != nil {
		t, err := ptypes.Timestamp(in.DisputeClosedAt)
		if err != nil {
			return nil, err
		}
		out.DisputeClosedAt = t
	}

	if in.Summary != nil {
		out.Summary = &MgoRoyaltyReportSummary{
			ProductsItems: in.Summary.ProductsItems,
			ProductsTotal: in.Summary.ProductsTotal,
		}

		if in.Summary.Corrections != nil {
			var list []*MgoRoyaltyReportCorrectionItem
			for _, v := range in.Summary.Corrections {
				item, err := NewRoyaltyReportCorrectionItemMapper().MapObjectToMgo(v)
				if err != nil {
					return nil, err
				}
				list = append(list, item.(*MgoRoyaltyReportCorrectionItem))
			}
			out.Summary.Corrections = list
		}

		if in.Summary.RollingReserves != nil {
			var list []*MgoRoyaltyReportCorrectionItem
			for _, v := range in.Summary.RollingReserves {
				item, err := NewRoyaltyReportCorrectionItemMapper().MapObjectToMgo(v)
				if err != nil {
					return nil, err
				}
				list = append(list, item.(*MgoRoyaltyReportCorrectionItem))
			}
			out.Summary.RollingReserves = list
		}
	}

	return out, nil
}

func (m *royaltyReportMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoRoyaltyReport)

	out := &billingpb.RoyaltyReport{
		Id:                 in.Id.Hex(),
		MerchantId:         in.MerchantId.Hex(),
		Status:             in.Status,
		Totals:             in.Totals,
		Currency:           in.Currency,
		DisputeReason:      in.DisputeReason,
		IsAutoAccepted:     in.IsAutoAccepted,
		PayoutDocumentId:   in.PayoutDocumentId,
		OperatingCompanyId: in.OperatingCompanyId,
		StringPeriodFrom:   in.StringPeriodFrom,
		StringPeriodTo:     in.StringPeriodTo,
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)
	if err != nil {
		return nil, err
	}

	out.UpdatedAt, err = ptypes.TimestampProto(in.UpdatedAt)
	if err != nil {
		return nil, err
	}

	out.PayoutDate, err = ptypes.TimestampProto(in.PayoutDate)
	if err != nil {
		return nil, err
	}

	out.PeriodFrom, err = ptypes.TimestampProto(in.PeriodFrom)
	if err != nil {
		return nil, err
	}

	out.PeriodTo, err = ptypes.TimestampProto(in.PeriodTo)
	if err != nil {
		return nil, err
	}

	out.AcceptExpireAt, err = ptypes.TimestampProto(in.AcceptExpireAt)
	if err != nil {
		return nil, err
	}

	out.AcceptedAt, err = ptypes.TimestampProto(in.AcceptedAt)
	if err != nil {
		return nil, err
	}

	out.DisputeStartedAt, err = ptypes.TimestampProto(in.DisputeStartedAt)
	if err != nil {
		return nil, err
	}

	out.DisputeClosedAt, err = ptypes.TimestampProto(in.DisputeClosedAt)
	if err != nil {
		return nil, err
	}

	if in.Summary != nil {
		out.Summary = &billingpb.RoyaltyReportSummary{
			ProductsItems: in.Summary.ProductsItems,
			ProductsTotal: in.Summary.ProductsTotal,
		}

		if in.Summary.Corrections != nil {
			var list []*billingpb.RoyaltyReportCorrectionItem
			for _, v := range in.Summary.Corrections {
				item, err := NewRoyaltyReportCorrectionItemMapper().MapMgoToObject(v)
				if err != nil {
					return nil, err
				}
				list = append(list, item.(*billingpb.RoyaltyReportCorrectionItem))
			}
			out.Summary.Corrections = list
		}

		if in.Summary.RollingReserves != nil {
			var list []*billingpb.RoyaltyReportCorrectionItem
			for _, v := range in.Summary.RollingReserves {
				item, err := NewRoyaltyReportCorrectionItemMapper().MapMgoToObject(v)
				if err != nil {
					return nil, err
				}
				list = append(list, item.(*billingpb.RoyaltyReportCorrectionItem))
			}
			out.Summary.RollingReserves = list
		}
	}

	if len(in.Merchants) > 0 {
		if in.Merchants[0].Company != nil {
			out.MerchantName = in.Merchants[0].Company.Name
		}
	}

	return out, nil
}

func (m *royaltyReportChangesMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.RoyaltyReportChanges)

	out := &MgoRoyaltyReportChanges{
		Source: in.Source,
		Ip:     in.Ip,
		Hash:   in.Hash,
	}

	if len(in.Id) <= 0 {
		out.Id = primitive.NewObjectID()
	} else {
		oid, err := primitive.ObjectIDFromHex(in.Id)
		if err != nil {
			return nil, err
		}
		out.Id = oid
	}

	if in.CreatedAt != nil {
		t, err := ptypes.Timestamp(in.CreatedAt)
		if err != nil {
			return nil, err
		}
		out.CreatedAt = t
	} else {
		out.CreatedAt = time.Now()
	}

	royaltyReportOid, err := primitive.ObjectIDFromHex(in.RoyaltyReportId)
	if err != nil {
		return nil, err
	}
	out.RoyaltyReportId = royaltyReportOid

	return out, nil
}

func (m *royaltyReportChangesMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoRoyaltyReportChanges)

	out := &billingpb.RoyaltyReportChanges{
		Id:              in.Id.Hex(),
		RoyaltyReportId: in.RoyaltyReportId.Hex(),
		Source:          in.Source,
		Ip:              in.Ip,
		Hash:            in.Hash,
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *royaltyReportCorrectionItemMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.RoyaltyReportCorrectionItem)

	out := &MgoRoyaltyReportCorrectionItem{
		Amount:   in.Amount,
		Currency: in.Currency,
		Reason:   in.Reason,
	}

	t, err := ptypes.Timestamp(in.EntryDate)
	if err != nil {
		return nil, err
	}
	out.EntryDate = t

	accountingEntryOid, err := primitive.ObjectIDFromHex(in.AccountingEntryId)
	if err != nil {
		return nil, err
	}
	out.AccountingEntryId = accountingEntryOid

	return out, nil
}

func (m *royaltyReportCorrectionItemMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoRoyaltyReportCorrectionItem)

	out := &billingpb.RoyaltyReportCorrectionItem{
		AccountingEntryId: in.AccountingEntryId.Hex(),
		Amount:            in.Amount,
		Currency:          in.Currency,
		Reason:            in.Reason,
	}

	out.EntryDate, err = ptypes.TimestampProto(in.EntryDate)
	if err != nil {
		return nil, err
	}

	return out, nil
}
