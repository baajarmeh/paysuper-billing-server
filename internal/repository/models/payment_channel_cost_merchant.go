package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type paymentChannelCostMerchantMapper struct{}

func NewPaymentChannelCostMerchantMapper() Mapper {
	return &paymentChannelCostMerchantMapper{}
}

type MgoPaymentChannelCostMerchant struct {
	Id                      primitive.ObjectID `bson:"_id" faker:"objectId"`
	MerchantId              primitive.ObjectID `bson:"merchant_id" faker:"objectId"`
	Name                    string             `bson:"name"`
	PayoutCurrency          string             `bson:"payout_currency"`
	MinAmount               float64            `bson:"min_amount"`
	MaxAmount               float64            `bson:"max_amount"`
	Region                  string             `bson:"region"`
	Country                 string             `bson:"country"`
	MethodPercent           float64            `bson:"method_percent"`
	MethodFixAmount         float64            `bson:"method_fix_amount"`
	MethodFixAmountCurrency string             `bson:"method_fix_amount_currency"`
	PsPercent               float64            `bson:"ps_percent"`
	PsFixedFee              float64            `bson:"ps_fixed_fee"`
	PsFixedFeeCurrency      string             `bson:"ps_fixed_fee_currency"`
	CreatedAt               time.Time          `bson:"created_at"`
	UpdatedAt               time.Time          `bson:"updated_at"`
	IsActive                bool               `bson:"is_active"`
	MccCode                 string             `bson:"mcc_code"`
}

func (m *paymentChannelCostMerchantMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.PaymentChannelCostMerchant)

	out := &MgoPaymentChannelCostMerchant{
		Name:                    in.Name,
		PayoutCurrency:          in.PayoutCurrency,
		MinAmount:               in.MinAmount,
		MaxAmount:               in.MaxAmount,
		Region:                  in.Region,
		Country:                 in.Country,
		MethodPercent:           in.MethodPercent,
		MethodFixAmount:         in.MethodFixAmount,
		MethodFixAmountCurrency: in.MethodFixAmountCurrency,
		PsPercent:               in.PsPercent,
		PsFixedFee:              in.PsFixedFee,
		PsFixedFeeCurrency:      in.PsFixedFeeCurrency,
		IsActive:                in.IsActive,
		MccCode:                 in.MccCode,
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

	return out, nil
}

func (m *paymentChannelCostMerchantMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoPaymentChannelCostMerchant)

	out := &billingpb.PaymentChannelCostMerchant{
		Id:                      in.Id.Hex(),
		MerchantId:              in.MerchantId.Hex(),
		Name:                    in.Name,
		PayoutCurrency:          in.PayoutCurrency,
		MinAmount:               in.MinAmount,
		MaxAmount:               in.MaxAmount,
		Region:                  in.Region,
		Country:                 in.Country,
		MethodPercent:           in.MethodPercent,
		MethodFixAmount:         in.MethodFixAmount,
		MethodFixAmountCurrency: in.MethodFixAmountCurrency,
		PsPercent:               in.PsPercent,
		PsFixedFee:              in.PsFixedFee,
		PsFixedFeeCurrency:      in.PsFixedFeeCurrency,
		IsActive:                in.IsActive,
		MccCode:                 in.MccCode,
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)

	if err != nil {
		return nil, err
	}

	out.UpdatedAt, err = ptypes.TimestampProto(in.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return out, nil
}
