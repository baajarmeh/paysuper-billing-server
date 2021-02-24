package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	tools "github.com/paysuper/paysuper-tools/number"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type MgoMultiLang struct {
	Lang  string `bson:"lang"`
	Value string `bson:"value"`
}

type MgoOrderProject struct {
	Id                      primitive.ObjectID                 `bson:"_id" faker:"objectId"`
	MerchantId              primitive.ObjectID                 `bson:"merchant_id" faker:"objectId"`
	Name                    []*MgoMultiLang                    `bson:"name"`
	UrlSuccess              string                             `bson:"url_success"`
	UrlFail                 string                             `bson:"url_fail"`
	NotifyEmails            []string                           `bson:"notify_emails"`
	SecretKey               string                             `bson:"secret_key"`
	SendNotifyEmail         bool                               `bson:"send_notify_email"`
	UrlCheckAccount         string                             `bson:"url_check_account"`
	UrlProcessPayment       string                             `bson:"url_process_payment"`
	CallbackProtocol        string                             `bson:"callback_protocol"`
	UrlChargebackPayment    string                             `bson:"url_chargeback_payment"`
	UrlCancelPayment        string                             `bson:"url_cancel_payment"`
	UrlFraudPayment         string                             `bson:"url_fraud_payment"`
	UrlRefundPayment        string                             `bson:"url_refund_payment"`
	Status                  int32                              `bson:"status"`
	MerchantRoyaltyCurrency string                             `bson:"merchant_royalty_currency"`
	RedirectSettings        *billingpb.ProjectRedirectSettings `bson:"redirect_settings"`
	FirstPaymentAt          time.Time                          `bson:"first_payment_at"`
	FormDefaultText         []*MgoMultiLang                    `bson:"form_default_text"`
}

type MgoOrderPaymentMethod struct {
	Id               primitive.ObjectID             `bson:"_id" faker:"objectId"`
	Name             string                         `bson:"name"`
	Handler          string                         `bson:"handler"`
	ExternalId       string                         `bson:"external_id"`
	Params           *billingpb.PaymentMethodParams `bson:"params"`
	PaymentSystemId  primitive.ObjectID             `bson:"payment_system_id" faker:"objectId"`
	Group            string                         `bson:"group_alias"`
	Saved            bool                           `bson:"saved"`
	Card             *billingpb.PaymentMethodCard   `bson:"card,omitempty"`
	Wallet           *billingpb.PaymentMethodWallet `bson:"wallet,omitempty"`
	CryptoCurrency   *billingpb.PaymentMethodCrypto `bson:"crypto_currency,omitempty"`
	RefundAllowed    bool                           `bson:"refund_allowed"`
	RecurringAllowed bool                           `bson:"recurring_allowed"`
}

type MgoOrderItem struct {
	Id          primitive.ObjectID `bson:"_id" faker:"objectId"`
	Object      string             `bson:"object"`
	Sku         string             `bson:"sku"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	Amount      float64            `bson:"amount"`
	Currency    string             `bson:"currency"`
	Images      []string           `bson:"images"`
	Url         string             `bson:"url"`
	Metadata    map[string]string  `bson:"metadata"`
	Code        string             `bson:"code"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	PlatformId  string             `bson:"platform_id"`
}

type MgoOrderViewReportSummary struct {
	Status  string                    `bson:"status" json:"status"`
	Charge  *billingpb.OrderViewMoney `bson:"charge" json:"charge"`
	Gross   *billingpb.OrderViewMoney `bson:"gross" json:"gross"`
	Vat     *billingpb.OrderViewMoney `bson:"vat" json:"vat"`
	Fees    *billingpb.OrderViewMoney `bson:"fees" json:"fees"`
	Revenue *billingpb.OrderViewMoney `bson:"revenue" json:"revenue"`
}

func getPaymentMethodOrder(in *MgoOrderPaymentMethod) *billingpb.PaymentMethodOrder {
	if in == nil {
		return nil
	}

	result := &billingpb.PaymentMethodOrder{
		Id:               in.Id.Hex(),
		Name:             in.Name,
		ExternalId:       in.ExternalId,
		Params:           in.Params,
		PaymentSystemId:  in.PaymentSystemId.Hex(),
		Group:            in.Group,
		Saved:            in.Saved,
		RefundAllowed:    in.RefundAllowed,
		Handler:          in.Handler,
		RecurringAllowed: in.RecurringAllowed,
	}

	if in.Card != nil {
		result.Card = in.Card
	}
	if in.Wallet != nil {
		result.Wallet = in.Wallet
	}
	if in.CryptoCurrency != nil {
		result.CryptoCurrency = in.CryptoCurrency
	}

	return result
}

func getOrderProject(in *MgoOrderProject) *billingpb.ProjectOrder {
	project := &billingpb.ProjectOrder{
		Id:                      in.Id.Hex(),
		MerchantId:              in.MerchantId.Hex(),
		UrlSuccess:              in.UrlSuccess,
		UrlFail:                 in.UrlFail,
		NotifyEmails:            in.NotifyEmails,
		SendNotifyEmail:         in.SendNotifyEmail,
		SecretKey:               in.SecretKey,
		UrlCheckAccount:         in.UrlCheckAccount,
		UrlProcessPayment:       in.UrlProcessPayment,
		UrlChargebackPayment:    in.UrlChargebackPayment,
		UrlCancelPayment:        in.UrlCancelPayment,
		UrlRefundPayment:        in.UrlRefundPayment,
		UrlFraudPayment:         in.UrlFraudPayment,
		CallbackProtocol:        in.CallbackProtocol,
		Status:                  in.Status,
		MerchantRoyaltyCurrency: in.MerchantRoyaltyCurrency,
		RedirectSettings:        in.RedirectSettings,
	}

	project.FirstPaymentAt, _ = ptypes.TimestampProto(in.FirstPaymentAt)

	if len(in.Name) > 0 {
		project.Name = make(map[string]string)

		for _, v := range in.Name {
			project.Name[v.Lang] = v.Value
		}
	}

	if len(in.FormDefaultText) > 0 {
		project.FormDefaultText = make(map[string]string)

		for _, v := range in.FormDefaultText {
			project.FormDefaultText[v.Lang] = v.Value
		}
	}

	return project
}

func getOrderViewMoney(in *billingpb.OrderViewMoney) *billingpb.OrderViewMoney {
	if in == nil {
		return &billingpb.OrderViewMoney{}
	}

	return &billingpb.OrderViewMoney{
		Amount:        tools.ToPrecise(in.Amount),
		Currency:      in.Currency,
		AmountRounded: in.AmountRounded,
	}
}

func getOrderViewItems(in []*MgoOrderItem) []*billingpb.OrderItem {
	var items []*billingpb.OrderItem

	if len(in) <= 0 {
		return items
	}

	for _, v := range in {
		item := &billingpb.OrderItem{
			Id:          v.Id.Hex(),
			Object:      v.Object,
			Sku:         v.Sku,
			Name:        v.Name,
			Description: v.Description,
			Amount:      v.Amount,
			Currency:    v.Currency,
			Images:      v.Images,
			Url:         v.Url,
			Metadata:    v.Metadata,
			Code:        v.Code,
			PlatformId:  v.PlatformId,
		}

		item.CreatedAt, _ = ptypes.TimestampProto(v.CreatedAt)
		item.CreatedAt, _ = ptypes.TimestampProto(v.UpdatedAt)

		items = append(items, item)
	}

	return items
}

func getOrderViewReportSummary(in *MgoOrderViewReportSummary) *billingpb.OrderViewReportSummary {
	if in == nil {
		return nil
	}

	result := &billingpb.OrderViewReportSummary{
		Status:  in.Status,
		Charge:  getOrderViewMoney(in.Charge),
		Gross:   getOrderViewMoney(in.Gross),
		Vat:     getOrderViewMoney(in.Vat),
		Fees:    getOrderViewMoney(in.Fees),
		Revenue: getOrderViewMoney(in.Revenue),
	}
	return result
}
