package models

import (
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type MgoProject struct {
	Id                       primitive.ObjectID `bson:"_id" json:"id"`
	MerchantId               primitive.ObjectID `bson:"merchant_id" json:"merchant_id"`
	Name                     []*MgoMultiLang    `bson:"name" json:"name"`
	CallbackCurrency         string             `bson:"callback_currency" json:"callback_currency"`
	CallbackProtocol         string             `bson:"callback_protocol" json:"callback_protocol"`
	CreateOrderAllowedUrls   []string           `bson:"create_order_allowed_urls" json:"create_order_allowed_urls"`
	AllowDynamicNotifyUrls   bool               `bson:"allow_dynamic_notify_urls" json:"allow_dynamic_notify_urls"`
	AllowDynamicRedirectUrls bool               `bson:"allow_dynamic_redirect_urls" json:"allow_dynamic_redirect_urls"`
	LimitsCurrency           string             `bson:"limits_currency" json:"limits_currency"`
	MinPaymentAmount         float64            `bson:"min_payment_amount" json:"min_payment_amount"`
	MaxPaymentAmount         float64            `bson:"max_payment_amount" json:"max_payment_amount"`
	NotifyEmails             []string           `bson:"notify_emails" json:"notify_emails"`
	IsProductsCheckout       bool               `bson:"is_products_checkout" json:"is_products_checkout"`
	SecretKey                string             `bson:"secret_key" json:"secret_key"`
	SignatureRequired        bool               `bson:"signature_required" json:"signature_required"`
	SendNotifyEmail          bool               `bson:"send_notify_email" json:"send_notify_email"`
	UrlCheckAccount          string             `bson:"url_check_account" json:"url_check_account"`
	UrlProcessPayment        string             `bson:"url_process_payment" json:"url_process_payment"`
	UrlRedirectFail          string             `bson:"url_redirect_fail" json:"url_redirect_fail"`
	UrlRedirectSuccess       string             `bson:"url_redirect_success" json:"url_redirect_success"`
	Status                   int32              `bson:"status" json:"status"`
	CreatedAt                time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt                time.Time          `bson:"updated_at" json:"updated_at"`
	ProductsCount            int64              `bson:"products_count" json:"products_count"`
	IdString                 string             `bson:"id_string" json:"id_string"`
	UrlChargebackPayment     string             `bson:"url_chargeback_payment" json:"url_chargeback_payment"`
	UrlCancelPayment         string             `bson:"url_cancel_payment" json:"url_cancel_payment"`
	UrlFraudPayment          string             `bson:"url_fraud_payment" json:"url_fraud_payment"`
	UrlRefundPayment         string             `bson:"url_refund_payment" json:"url_refund_payment"`

	Cover            *billingpb.ImageCollection         `bson:"cover" json:"cover"`
	Localizations    []string                           `bson:"localizations" json:"localizations"`
	FullDescription  map[string]string                  `bson:"full_description" json:"full_description"`
	ShortDescription map[string]string                  `bson:"short_description" json:"short_description"`
	Currencies       []*billingpb.HasCurrencyItem       `bson:"currencies" json:"currencies"`
	VirtualCurrency  *billingpb.ProjectVirtualCurrency  `bson:"virtual_currency" json:"virtual_currency"`
	VatPayer         string                             `bson:"vat_payer" json:"vat_payer"`
	RedirectSettings *billingpb.ProjectRedirectSettings `bson:"redirect_settings" json:"redirect_settings"`
	WebHookMode      string                             `bson:"webhook_mode" json:"webhook_mode"`
	WebhookTesting   *billingpb.WebHookTesting          `bson:"webhook_testing" json:"webhook_testing"`
	FormDefaultText  []*MgoMultiLang                    `bson:"form_default_text" json:"form_default_text"`
}

type projectMapper struct {
}

func (p *projectMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	m := obj.(*billingpb.Project)
	merchantOid, err := primitive.ObjectIDFromHex(m.MerchantId)

	if err != nil {
		return nil, err
	}

	st := &MgoProject{
		MerchantId:               merchantOid,
		CallbackCurrency:         m.CallbackCurrency,
		CallbackProtocol:         m.CallbackProtocol,
		CreateOrderAllowedUrls:   m.CreateOrderAllowedUrls,
		AllowDynamicNotifyUrls:   m.AllowDynamicNotifyUrls,
		AllowDynamicRedirectUrls: m.AllowDynamicRedirectUrls,
		LimitsCurrency:           m.LimitsCurrency,
		MaxPaymentAmount:         m.MaxPaymentAmount,
		MinPaymentAmount:         m.MinPaymentAmount,
		NotifyEmails:             m.NotifyEmails,
		IsProductsCheckout:       m.IsProductsCheckout,
		SecretKey:                m.SecretKey,
		SignatureRequired:        m.SignatureRequired,
		SendNotifyEmail:          m.SendNotifyEmail,
		UrlCheckAccount:          m.UrlCheckAccount,
		UrlProcessPayment:        m.UrlProcessPayment,
		UrlRedirectFail:          m.UrlRedirectFail,
		UrlRedirectSuccess:       m.UrlRedirectSuccess,
		Status:                   m.Status,
		UrlChargebackPayment:     m.UrlChargebackPayment,
		UrlCancelPayment:         m.UrlCancelPayment,
		UrlFraudPayment:          m.UrlFraudPayment,
		UrlRefundPayment:         m.UrlRefundPayment,
		Cover:                    m.Cover,
		Localizations:            m.Localizations,
		FullDescription:          m.FullDescription,
		ShortDescription:         m.ShortDescription,
		Currencies:               m.Currencies,
		VirtualCurrency:          m.VirtualCurrency,
		VatPayer:                 m.VatPayer,
		RedirectSettings:         m.RedirectSettings,
		WebHookMode:              m.WebhookMode,
		WebhookTesting:           m.WebhookTesting,
	}

	if len(m.Name) > 0 {
		for k, v := range m.Name {
			st.Name = append(st.Name, &MgoMultiLang{Lang: k, Value: v})
		}
	}

	if len(m.FormDefaultText) > 0 {
		for k, v := range m.FormDefaultText {
			st.FormDefaultText = append(st.FormDefaultText, &MgoMultiLang{Lang: k, Value: v})
		}
	}

	if len(m.Id) <= 0 {
		st.Id = primitive.NewObjectID()
	} else {
		oid, err := primitive.ObjectIDFromHex(m.Id)

		if err != nil {
			return nil, errors.New(billingpb.ErrorInvalidObjectId)
		}

		st.Id = oid
	}

	st.IdString = st.Id.Hex()

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (p *projectMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	decoded := obj.(*MgoProject)

	m := &billingpb.Project{}
	m.Id = decoded.Id.Hex()
	m.MerchantId = decoded.MerchantId.Hex()
	m.CallbackCurrency = decoded.CallbackCurrency
	m.CallbackProtocol = decoded.CallbackProtocol
	m.CreateOrderAllowedUrls = decoded.CreateOrderAllowedUrls
	m.AllowDynamicNotifyUrls = decoded.AllowDynamicNotifyUrls
	m.AllowDynamicRedirectUrls = decoded.AllowDynamicRedirectUrls
	m.LimitsCurrency = decoded.LimitsCurrency
	m.MaxPaymentAmount = decoded.MaxPaymentAmount
	m.MinPaymentAmount = decoded.MinPaymentAmount
	m.NotifyEmails = decoded.NotifyEmails
	m.IsProductsCheckout = decoded.IsProductsCheckout
	m.SecretKey = decoded.SecretKey
	m.SignatureRequired = decoded.SignatureRequired
	m.SendNotifyEmail = decoded.SendNotifyEmail
	m.UrlCheckAccount = decoded.UrlCheckAccount
	m.UrlProcessPayment = decoded.UrlProcessPayment
	m.UrlRedirectFail = decoded.UrlRedirectFail
	m.UrlRedirectSuccess = decoded.UrlRedirectSuccess
	m.Status = decoded.Status
	m.UrlChargebackPayment = decoded.UrlChargebackPayment
	m.UrlCancelPayment = decoded.UrlCancelPayment
	m.UrlFraudPayment = decoded.UrlFraudPayment
	m.UrlRefundPayment = decoded.UrlRefundPayment
	m.Cover = decoded.Cover
	m.Localizations = decoded.Localizations
	m.FullDescription = decoded.FullDescription
	m.ShortDescription = decoded.ShortDescription
	m.Currencies = decoded.Currencies
	m.VirtualCurrency = decoded.VirtualCurrency
	m.VatPayer = decoded.VatPayer
	m.RedirectSettings = decoded.RedirectSettings
	m.WebhookMode = decoded.WebHookMode
	m.WebhookTesting = decoded.WebhookTesting

	nameLen := len(decoded.Name)

	if nameLen > 0 {
		m.Name = make(map[string]string, nameLen)

		for _, v := range decoded.Name {
			m.Name[v.Lang] = v.Value
		}
	}

	formDefaultTextLen := len(decoded.FormDefaultText)

	if formDefaultTextLen > 0 {
		m.FormDefaultText = make(map[string]string, formDefaultTextLen)

		for _, v := range decoded.FormDefaultText {
			m.FormDefaultText[v.Lang] = v.Value
		}
	}

	if decoded.ProductsCount > 0 {
		m.ProductsCount = decoded.ProductsCount
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return nil, err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return m, nil
}

func NewProjectMapper() Mapper {
	return &projectMapper{}
}
