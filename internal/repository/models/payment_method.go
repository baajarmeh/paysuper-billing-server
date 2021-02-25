package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/helper"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type paymentMethodMapper struct{}

func NewPaymentMethodMapper() Mapper {
	return &paymentMethodMapper{}
}

type MgoPaymentMethod struct {
	Id                 primitive.ObjectID       `bson:"_id" faker:"objectId"`
	Name               string                   `bson:"name"`
	Group              string                   `bson:"group_alias"`
	ExternalId         string                   `bson:"external_id"`
	LimitsCurrency     string                   `bson:"limits_currency"`
	MinPaymentAmount   float64                  `bson:"min_payment_amount"`
	MaxPaymentAmount   float64                  `bson:"max_payment_amount"`
	TestSettings       []*MgoPaymentMethodParam `bson:"test_settings"`
	ProductionSettings []*MgoPaymentMethodParam `bson:"production_settings"`
	IsActive           bool                     `bson:"is_active"`
	CreatedAt          time.Time                `bson:"created_at"`
	UpdatedAt          time.Time                `bson:"updated_at"`
	PaymentSystemId    primitive.ObjectID       `bson:"payment_system_id" faker:"objectId"`
	Currencies         []string                 `bson:"currencies"`
	Type               string                   `bson:"type"`
	AccountRegexp      string                   `bson:"account_regexp"`
	RefundAllowed      bool                     `bson:"refund_allowed"`
	RecurringAllowed   bool                     `bson:"recurring_allowed"`
}

type MgoPaymentMethodParam struct {
	TerminalId         string   `bson:"terminal_id"`
	Secret             string   `bson:"secret"`
	SecretCallback     string   `bson:"secret_callback"`
	Currency           string   `bson:"currency"`
	ApiUrl             string   `bson:"api_url"`
	MccCode            string   `bson:"mcc_code"`
	OperatingCompanyId string   `bson:"operating_company_id" faker:"objectId"`
	Brand              []string `bson:"brand"`
}

func (m *paymentMethodMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.PaymentMethod)

	out := &MgoPaymentMethod{
		Name:             in.Name,
		Group:            in.Group,
		ExternalId:       in.ExternalId,
		LimitsCurrency:   in.LimitsCurrency,
		MinPaymentAmount: in.MinPaymentAmount,
		MaxPaymentAmount: in.MaxPaymentAmount,
		Type:             in.Type,
		AccountRegexp:    in.AccountRegexp,
		IsActive:         in.IsActive,
		RefundAllowed:    in.RefundAllowed,
		RecurringAllowed: in.RecurringAllowed,
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

	if in.PaymentSystemId != "" {
		oid, err := primitive.ObjectIDFromHex(in.PaymentSystemId)

		if err != nil {
			return nil, err
		}

		out.PaymentSystemId = oid
	}

	if in.TestSettings != nil {
		check := make(map[string]bool)

		for _, value := range in.TestSettings {

			key := helper.GetPaymentMethodKey(value.Currency, value.MccCode, value.OperatingCompanyId, "")

			if check[key] == true {
				continue
			}

			check[key] = true

			out.TestSettings = append(out.TestSettings, &MgoPaymentMethodParam{
				Currency:           value.Currency,
				TerminalId:         value.TerminalId,
				Secret:             value.Secret,
				SecretCallback:     value.SecretCallback,
				ApiUrl:             value.ApiUrl,
				MccCode:            value.MccCode,
				OperatingCompanyId: value.OperatingCompanyId,
				Brand:              value.Brand,
			})
		}
	}

	if in.ProductionSettings != nil {
		check := make(map[string]bool)

		for _, value := range in.ProductionSettings {

			key := helper.GetPaymentMethodKey(value.Currency, value.MccCode, value.OperatingCompanyId, "")

			if check[key] == true {
				continue
			}

			check[key] = true

			out.ProductionSettings = append(out.ProductionSettings, &MgoPaymentMethodParam{
				Currency:           value.Currency,
				TerminalId:         value.TerminalId,
				Secret:             value.Secret,
				SecretCallback:     value.SecretCallback,
				ApiUrl:             value.ApiUrl,
				MccCode:            value.MccCode,
				OperatingCompanyId: value.OperatingCompanyId,
				Brand:              value.Brand,
			})
		}
	}

	return out, nil
}

func (m *paymentMethodMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoPaymentMethod)

	out := &billingpb.PaymentMethod{
		Id:               in.Id.Hex(),
		Name:             in.Name,
		Group:            in.Group,
		ExternalId:       in.ExternalId,
		LimitsCurrency:   in.LimitsCurrency,
		MinPaymentAmount: in.MinPaymentAmount,
		MaxPaymentAmount: in.MaxPaymentAmount,
		Type:             in.Type,
		AccountRegexp:    in.AccountRegexp,
		IsActive:         in.IsActive,
		RefundAllowed:    in.RefundAllowed,
		RecurringAllowed: in.RecurringAllowed,
		PaymentSystemId:  in.PaymentSystemId.Hex(),
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)

	if err != nil {
		return nil, err
	}

	out.UpdatedAt, err = ptypes.TimestampProto(in.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if in.TestSettings != nil {
		pmp := make(map[string]*billingpb.PaymentMethodParams, len(in.TestSettings))
		for _, value := range in.TestSettings {
			if value == nil {
				continue
			}

			params := &billingpb.PaymentMethodParams{
				Currency:           value.Currency,
				TerminalId:         value.TerminalId,
				Secret:             value.Secret,
				SecretCallback:     value.SecretCallback,
				ApiUrl:             value.ApiUrl,
				MccCode:            value.MccCode,
				OperatingCompanyId: value.OperatingCompanyId,
				Brand:              value.Brand,
			}

			key := helper.GetPaymentMethodKey(value.Currency, value.MccCode, value.OperatingCompanyId, "")
			pmp[key] = params

			for _, brand := range value.Brand {
				key := helper.GetPaymentMethodKey(value.Currency, value.MccCode, value.OperatingCompanyId, brand)
				pmp[key] = params
			}

		}
		out.TestSettings = pmp
	}

	if in.ProductionSettings != nil {
		pmp := make(map[string]*billingpb.PaymentMethodParams, len(in.ProductionSettings))
		for _, value := range in.ProductionSettings {
			if value == nil {
				continue
			}

			params := &billingpb.PaymentMethodParams{
				Currency:           value.Currency,
				TerminalId:         value.TerminalId,
				Secret:             value.Secret,
				SecretCallback:     value.SecretCallback,
				ApiUrl:             value.ApiUrl,
				MccCode:            value.MccCode,
				OperatingCompanyId: value.OperatingCompanyId,
				Brand:              value.Brand,
			}

			key := helper.GetPaymentMethodKey(value.Currency, value.MccCode, value.OperatingCompanyId, "")
			pmp[key] = params

			for _, brand := range value.Brand {
				key := helper.GetPaymentMethodKey(value.Currency, value.MccCode, value.OperatingCompanyId, brand)
				pmp[key] = params
			}
		}
		out.ProductionSettings = pmp
	}

	return out, nil
}
