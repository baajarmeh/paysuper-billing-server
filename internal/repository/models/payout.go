package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type payoutMapper struct{}

type payoutChangesMapper struct{}

func NewPayoutMapper() Mapper {
	return &payoutMapper{}
}

func NewPayoutChangesMapper() Mapper {
	return &payoutChangesMapper{}
}

type MgoPayoutDocument struct {
	Id                      primitive.ObjectID             `bson:"_id" faker:"objectId"`
	MerchantId              primitive.ObjectID             `bson:"merchant_id" faker:"objectId"`
	SourceId                []string                       `bson:"source_id"`
	TotalFees               float64                        `bson:"total_fees"`
	Balance                 float64                        `bson:"balance"`
	Currency                string                         `bson:"currency"`
	PeriodFrom              time.Time                      `bson:"period_from"`
	PeriodTo                time.Time                      `bson:"period_to"`
	TotalTransactions       int32                          `bson:"total_transactions"`
	Description             string                         `bson:"description"`
	Destination             *billingpb.MerchantBanking     `bson:"destination"`
	MerchantAgreementNumber string                         `bson:"merchant_agreement_number"`
	Company                 *billingpb.MerchantCompanyInfo `bson:"company"`
	Status                  string                         `bson:"status"`
	Transaction             string                         `bson:"transaction"`
	FailureCode             string                         `bson:"failure_code"`
	FailureMessage          string                         `bson:"failure_message"`
	FailureTransaction      string                         `bson:"failure_transaction"`
	CreatedAt               time.Time                      `bson:"created_at"`
	UpdatedAt               time.Time                      `bson:"updated_at"`
	ArrivalDate             time.Time                      `bson:"arrival_date"`
	PaidAt                  time.Time                      `bson:"paid_at"`
	OperatingCompanyId      string                         `bson:"operating_company_id"`
	AutoincrementId         int64                          `bson:"autoincrement_id"`
	StringPeriodFrom        string                         `bson:"string_period_from"`
	StringPeriodTo          string                         `bson:"string_period_to"`
	B2BVatBase              float64                        `bson:"b2b_vat_base"`
	B2BVatRate              float64                        `bson:"b2b_vat_rate"`
	B2BVatAmount            float64                        `bson:"b2b_vat_amount"`
	FeesExcludingVat        float64                        `bson:"fees_excluding_vat"`
}

type MgoPayoutDocumentChanges struct {
	Id               primitive.ObjectID `bson:"_id" faker:"objectId"`
	PayoutDocumentId primitive.ObjectID `bson:"payout_document_id" faker:"objectId"`
	Source           string             `bson:"source"`
	Hash             string             `bson:"hash"`
	Ip               string             `bson:"ip"`
	CreatedAt        time.Time          `bson:"created_at"`
}

func (m *payoutMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.PayoutDocument)

	out := &MgoPayoutDocument{
		SourceId:                in.SourceId,
		TotalFees:               in.TotalFees,
		Balance:                 in.Balance,
		Currency:                in.Currency,
		TotalTransactions:       in.TotalTransactions,
		Description:             in.Description,
		MerchantAgreementNumber: in.MerchantAgreementNumber,
		Status:                  in.Status,
		Transaction:             in.Transaction,
		FailureCode:             in.FailureCode,
		FailureMessage:          in.FailureMessage,
		FailureTransaction:      in.FailureTransaction,
		Destination:             in.Destination,
		Company:                 in.Company,
		OperatingCompanyId:      in.OperatingCompanyId,
		AutoincrementId:         in.AutoincrementId,
		StringPeriodFrom:        in.StringPeriodFrom,
		StringPeriodTo:          in.StringPeriodTo,
		B2BVatBase:              in.B2BVatBase,
		B2BVatRate:              in.B2BVatRate,
		B2BVatAmount:            in.B2BVatAmount,
		FeesExcludingVat:        in.FeesExcludingVat,
	}

	merchantOid, err := primitive.ObjectIDFromHex(in.MerchantId)

	if err != nil {
		return nil, err
	}

	out.MerchantId = merchantOid

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

	if in.ArrivalDate != nil {
		t, err := ptypes.Timestamp(in.ArrivalDate)

		if err != nil {
			return nil, err
		}

		out.ArrivalDate = t
	} else {
		out.ArrivalDate = time.Now()
	}

	if in.PeriodFrom != nil {
		t, err := ptypes.Timestamp(in.PeriodFrom)

		if err != nil {
			return nil, err
		}

		out.PeriodFrom = t
	}

	if in.PeriodTo != nil {
		t, err := ptypes.Timestamp(in.PeriodTo)

		if err != nil {
			return nil, err
		}

		out.PeriodTo = t
	}

	if in.PaidAt != nil {
		t, err := ptypes.Timestamp(in.PaidAt)

		if err != nil {
			return nil, err
		}

		out.PaidAt = t
	}

	return out, nil
}

func (m *payoutMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoPayoutDocument)

	out := &billingpb.PayoutDocument{
		Id:                      in.Id.Hex(),
		MerchantId:              in.MerchantId.Hex(),
		SourceId:                in.SourceId,
		TotalFees:               in.TotalFees,
		Balance:                 in.Balance,
		Currency:                in.Currency,
		TotalTransactions:       in.TotalTransactions,
		Description:             in.Description,
		MerchantAgreementNumber: in.MerchantAgreementNumber,
		Status:                  in.Status,
		Transaction:             in.Transaction,
		FailureCode:             in.FailureCode,
		FailureMessage:          in.FailureMessage,
		FailureTransaction:      in.FailureTransaction,
		Destination:             in.Destination,
		Company:                 in.Company,
		OperatingCompanyId:      in.OperatingCompanyId,
		AutoincrementId:         in.AutoincrementId,
		StringPeriodFrom:        in.StringPeriodFrom,
		StringPeriodTo:          in.StringPeriodTo,
		B2BVatBase:              in.B2BVatBase,
		B2BVatRate:              in.B2BVatRate,
		B2BVatAmount:            in.B2BVatAmount,
		FeesExcludingVat:        in.FeesExcludingVat,
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)
	if err != nil {
		return nil, err
	}

	out.UpdatedAt, err = ptypes.TimestampProto(in.UpdatedAt)
	if err != nil {
		return nil, err
	}

	out.ArrivalDate, err = ptypes.TimestampProto(in.ArrivalDate)
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

	out.PaidAt, err = ptypes.TimestampProto(in.PaidAt)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *payoutChangesMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.PayoutDocumentChanges)

	out := &MgoPayoutDocumentChanges{
		Source: in.Source,
		Hash:   in.Hash,
		Ip:     in.Ip,
	}

	payoutDocumentOid, err := primitive.ObjectIDFromHex(in.PayoutDocumentId)

	if err != nil {
		return nil, err
	}

	out.PayoutDocumentId = payoutDocumentOid

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

	return out, nil
}

func (m *payoutChangesMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoPayoutDocumentChanges)

	out := &billingpb.PayoutDocumentChanges{
		Id:               in.Id.Hex(),
		PayoutDocumentId: in.PayoutDocumentId.Hex(),
		Source:           in.Source,
		Hash:             in.Hash,
		Ip:               in.Ip,
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)
	if err != nil {
		return nil, err
	}

	return out, nil
}
