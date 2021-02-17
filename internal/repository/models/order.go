package models

import (
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/recurringpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type MgoOrderNotificationRefund struct {
	Amount        float64 `bson:"amount"`
	Currency      string  `bson:"currency"`
	Reason        string  `bson:"reason"`
	Code          string  `bson:"code"`
	ReceiptNumber string  `bson:"receipt_number"`
	ReceiptUrl    string  `bson:"receipt_url"`
}

type MgoOrder struct {
	Id                          primitive.ObjectID                       `bson:"_id" faker:"objectId"`
	Uuid                        string                                   `bson:"uuid"`
	Transaction                 string                                   `bson:"pm_order_id"`
	Object                      string                                   `bson:"object"`
	Status                      string                                   `bson:"status"`
	PrivateStatus               int32                                    `bson:"private_status"`
	Description                 string                                   `bson:"description"`
	CreatedAt                   time.Time                                `bson:"created_at"`
	UpdatedAt                   time.Time                                `bson:"updated_at"`
	CanceledAt                  time.Time                                `bson:"canceled_at"`
	Canceled                    bool                                     `bson:"canceled"`
	Cancellation                *billingpb.OrderNotificationCancellation `bson:"cancellation"`
	Refunded                    bool                                     `bson:"refunded"`
	RefundedAt                  time.Time                                `bson:"refunded_at"`
	ReceiptEmail                string                                   `bson:"receipt_email"`
	ReceiptPhone                string                                   `bson:"receipt_phone"`
	ReceiptNumber               string                                   `bson:"receipt_number"`
	ReceiptUrl                  string                                   `bson:"receipt_url"`
	AgreementVersion            string                                   `bson:"agreement_version"`
	AgreementAccepted           bool                                     `bson:"agreement_accepted"`
	NotifySale                  bool                                     `bson:"notify_sale"`
	NotifySaleEmail             string                                   `bson:"notify_sale_email"`
	Issuer                      *billingpb.OrderIssuer                   `bson:"issuer"`
	TotalPaymentAmount          float64                                  `bson:"total_payment_amount"`
	Currency                    string                                   `bson:"currency"`
	User                        *billingpb.OrderUser                     `bson:"user"`
	BillingAddress              *billingpb.OrderBillingAddress           `bson:"billing_address"`
	Tax                         *billingpb.OrderTax                      `bson:"tax"`
	PaymentMethod               *MgoOrderPaymentMethod                   `bson:"payment_method"`
	Items                       []*MgoOrderItem                          `bson:"items"`
	Refund                      *MgoOrderNotificationRefund              `bson:"refund"`
	Metadata                    map[string]string                        `bson:"metadata"`
	PrivateMetadata             map[string]string                        `bson:"private_metadata"`
	Project                     *MgoOrderProject                         `bson:"project"`
	ProjectLastRequestedAt      time.Time                                `bson:"project_last_requested_at"`
	ProjectParams               map[string]string                        `bson:"project_params"`
	PaymentMethodOrderClosedAt  time.Time                                `bson:"pm_order_close_date"`
	IsJsonRequest               bool                                     `bson:"created_by_json"`
	OrderAmount                 float64                                  `bson:"private_amount"`
	PaymentMethodPayerAccount   string                                   `bson:"pm_account"`
	PaymentMethodTxnParams      map[string]string                        `bson:"pm_txn_params"`
	PaymentRequisites           map[string]string                        `bson:"payment_requisites"`
	ExpireDateToFormInput       time.Time                                `bson:"expire_date_to_form_input"`
	UserAddressDataRequired     bool                                     `bson:"user_address_data_required"`
	Products                    []string                                 `bson:"products"`
	IsNotificationsSent         map[string]bool                          `bson:"is_notifications_sent"`
	CountryRestriction          *billingpb.CountryRestriction            `bson:"country_restriction"`
	ParentOrder                 *billingpb.ParentOrder                   `bson:"parent_order"`
	ParentPaymentAt             time.Time                                `bson:"parent_payment_at"`
	Type                        string                                   `bson:"type"`
	IsVatDeduction              bool                                     `bson:"is_vat_deduction"`
	CountryCode                 string                                   `bson:"country_code"`
	PlatformId                  string                                   `bson:"platform_id"`
	ProductType                 string                                   `bson:"product_type"`
	Keys                        []string                                 `bson:"keys"`
	IsKeyProductNotified        bool                                     `bson:"is_key_product_notified"`
	ReceiptId                   string                                   `bson:"receipt_id"`
	IsBuyForVirtualCurrency     bool                                     `bson:"is_buy_for_virtual_currency"`
	MccCode                     string                                   `bson:"mcc_code"`
	OperatingCompanyId          string                                   `bson:"operating_company_id"`
	IsHighRisk                  bool                                     `bson:"is_high_risk"`
	ChargeCurrency              string                                   `bson:"charge_currency"`
	ChargeAmount                float64                                  `bson:"charge_amount"`
	PaymentIpCountry            string                                   `bson:"payment_ip_country"`
	IsIpCountryMismatchBin      bool                                     `bson:"is_ip_country_mismatch_bin"`
	BillingCountryChangedByUser bool                                     `bson:"billing_country_changed_by_user"`
	IsRefundAllowed             bool                                     `bson:"is_refund_allowed"`
	VatPayer                    string                                   `bson:"vat_payer"`
	IsProduction                bool                                     `bson:"is_production"`
	FormMode                    string                                   `bson:"form_mode"`
	IsCurrencyPredefined        bool                                     `bson:"is_currency_predefined"`
	MetadataValues              []string                                 `bson:"metadata_values" json:"-"`
	MerchantInfo                *billingpb.OrderViewMerchantInfo         `bson:"merchant_info"`
	RoyaltyReportId             string                                   `bson:"royalty_report_id"`
	NetRevenue                  *billingpb.OrderViewMoney                `bson:"-"`
	Fee                         *billingpb.OrderViewMoney                `bson:"-"`
	RecurringPlanId             string                                   `bson:"recurring_plan_id"`
	RecurringSubscriptionId     string                                   `bson:"recurring_subscription_id"`
}

type orderMapper struct{}

func (o *orderMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	m := obj.(*billingpb.Order)
	projectOid, err := primitive.ObjectIDFromHex(m.Project.Id)

	if err != nil {
		return nil, err
	}

	merchantOid, err := primitive.ObjectIDFromHex(m.Project.MerchantId)

	if err != nil {
		return nil, err
	}

	st := &MgoOrder{
		Uuid:                 m.Uuid,
		Transaction:          m.Transaction,
		Object:               "order",
		Status:               m.GetPublicStatus(),
		PrivateStatus:        m.PrivateStatus,
		Description:          m.Description,
		Canceled:             m.PrivateStatus == recurringpb.OrderStatusPaymentSystemCanceled,
		Cancellation:         m.Cancellation,
		Refunded:             m.PrivateStatus == recurringpb.OrderStatusRefund,
		ReceiptEmail:         m.GetReceiptUserEmail(),
		ReceiptPhone:         m.GetReceiptUserPhone(),
		ReceiptNumber:        m.ReceiptNumber,
		ReceiptUrl:           m.ReceiptUrl,
		AgreementVersion:     m.AgreementVersion,
		AgreementAccepted:    m.AgreementAccepted,
		NotifySale:           m.NotifySale,
		NotifySaleEmail:      m.NotifySaleEmail,
		Issuer:               m.Issuer,
		TotalPaymentAmount:   m.TotalPaymentAmount,
		Currency:             m.Currency,
		User:                 m.User,
		BillingAddress:       m.BillingAddress,
		Tax:                  m.Tax,
		Items:                []*MgoOrderItem{},
		Metadata:             m.Metadata,
		PrivateMetadata:      m.PrivateMetadata,
		IsCurrencyPredefined: m.IsCurrencyPredefined,
		Project: &MgoOrderProject{
			Id:                      projectOid,
			MerchantId:              merchantOid,
			UrlSuccess:              m.Project.UrlSuccess,
			UrlFail:                 m.Project.UrlFail,
			NotifyEmails:            m.Project.NotifyEmails,
			SendNotifyEmail:         m.Project.SendNotifyEmail,
			SecretKey:               m.Project.SecretKey,
			UrlCheckAccount:         m.Project.UrlCheckAccount,
			UrlProcessPayment:       m.Project.UrlProcessPayment,
			CallbackProtocol:        m.Project.CallbackProtocol,
			UrlChargebackPayment:    m.Project.UrlChargebackPayment,
			UrlCancelPayment:        m.Project.UrlCancelPayment,
			UrlRefundPayment:        m.Project.UrlRefundPayment,
			UrlFraudPayment:         m.Project.UrlFraudPayment,
			Status:                  m.Project.Status,
			MerchantRoyaltyCurrency: m.Project.MerchantRoyaltyCurrency,
			RedirectSettings:        m.Project.RedirectSettings,
		},
		ProjectParams:               m.ProjectParams,
		IsJsonRequest:               m.IsJsonRequest,
		OrderAmount:                 m.OrderAmount,
		PaymentMethodPayerAccount:   m.PaymentMethodPayerAccount,
		PaymentMethodTxnParams:      m.PaymentMethodTxnParams,
		PaymentRequisites:           m.PaymentRequisites,
		UserAddressDataRequired:     m.UserAddressDataRequired,
		Products:                    m.Products,
		IsNotificationsSent:         m.IsNotificationsSent,
		CountryRestriction:          m.CountryRestriction,
		ParentOrder:                 m.ParentOrder,
		Type:                        m.Type,
		IsVatDeduction:              m.IsVatDeduction,
		CountryCode:                 m.GetCountry(),
		ProductType:                 m.ProductType,
		PlatformId:                  m.PlatformId,
		Keys:                        m.Keys,
		IsKeyProductNotified:        m.IsKeyProductNotified,
		ReceiptId:                   m.ReceiptId,
		IsBuyForVirtualCurrency:     m.IsBuyForVirtualCurrency,
		MccCode:                     m.MccCode,
		OperatingCompanyId:          m.OperatingCompanyId,
		IsHighRisk:                  m.IsHighRisk,
		ChargeCurrency:              m.ChargeCurrency,
		ChargeAmount:                m.ChargeAmount,
		PaymentIpCountry:            m.PaymentIpCountry,
		IsIpCountryMismatchBin:      m.IsIpCountryMismatchBin,
		BillingCountryChangedByUser: m.BillingCountryChangedByUser,
		IsRefundAllowed:             m.IsRefundAllowed,
		VatPayer:                    m.VatPayer,
		IsProduction:                m.IsProduction,
		FormMode:                    m.FormMode,
		MerchantInfo:                m.MerchantInfo,
		RoyaltyReportId:             m.RoyaltyReportId,
		NetRevenue:                  m.NetRevenue,
		Fee:                         m.Fee,
		RecurringPlanId:             m.RecurringPlanId,
		RecurringSubscriptionId:     m.RecurringSubscriptionId,
	}

	if m.Refund != nil {
		st.Refund = &MgoOrderNotificationRefund{
			Amount:        m.Refund.Amount,
			Currency:      m.Refund.Currency,
			Reason:        m.Refund.Reason,
			Code:          m.Refund.Code,
			ReceiptNumber: m.Refund.ReceiptNumber,
			ReceiptUrl:    m.Refund.ReceiptUrl,
		}
	}

	for _, v := range m.Items {
		item := &MgoOrderItem{
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

		if len(v.Id) <= 0 {
			item.Id = primitive.NewObjectID()
		} else {
			itemOid, err := primitive.ObjectIDFromHex(v.Id)

			if err != nil {
				return nil, errors.New(billingpb.ErrorInvalidObjectId)
			}
			item.Id = itemOid
		}

		item.CreatedAt, err = ptypes.Timestamp(v.CreatedAt)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = ptypes.Timestamp(v.UpdatedAt)
		if err != nil {
			return nil, err
		}
		st.Items = append(st.Items, item)
	}

	if m.PaymentMethod != nil {
		paymentMethodOid, err := primitive.ObjectIDFromHex(m.PaymentMethod.Id)

		if err != nil {
			return nil, err
		}

		paymentSystemOid, err := primitive.ObjectIDFromHex(m.PaymentMethod.PaymentSystemId)

		if err != nil {
			return nil, err
		}

		st.PaymentMethod = &MgoOrderPaymentMethod{
			Id:               paymentMethodOid,
			Name:             m.PaymentMethod.Name,
			ExternalId:       m.PaymentMethod.ExternalId,
			Params:           m.PaymentMethod.Params,
			PaymentSystemId:  paymentSystemOid,
			Group:            m.PaymentMethod.Group,
			Saved:            m.PaymentMethod.Saved,
			Handler:          m.PaymentMethod.Handler,
			RefundAllowed:    m.PaymentMethod.RefundAllowed,
			RecurringAllowed: m.PaymentMethod.RecurringAllowed,
		}

		if m.PaymentMethod.Card != nil {
			st.PaymentMethod.Card = m.PaymentMethod.Card
		}
		if m.PaymentMethod.Wallet != nil {
			st.PaymentMethod.Wallet = m.PaymentMethod.Wallet
		}
		if m.PaymentMethod.CryptoCurrency != nil {
			st.PaymentMethod.CryptoCurrency = m.PaymentMethod.CryptoCurrency
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

	if m.ProjectLastRequestedAt != nil {
		t, err := ptypes.Timestamp(m.ProjectLastRequestedAt)

		if err != nil {
			return nil, err
		}

		st.ProjectLastRequestedAt = t
	}

	if m.PaymentMethodOrderClosedAt != nil {
		t, err := ptypes.Timestamp(m.PaymentMethodOrderClosedAt)

		if err != nil {
			return nil, err
		}

		st.PaymentMethodOrderClosedAt = t
	}

	if m.ExpireDateToFormInput != nil {
		t, err := ptypes.Timestamp(m.ExpireDateToFormInput)

		if err != nil {
			return nil, err
		}

		st.ExpireDateToFormInput = t
	} else {
		st.ExpireDateToFormInput = time.Now()
	}

	if m.Project != nil && len(m.Project.Name) > 0 {
		for k, v := range m.Project.Name {
			st.Project.Name = append(st.Project.Name, &MgoMultiLang{Lang: k, Value: v})
		}
	}

	if m.Project != nil && len(m.Project.FormDefaultText) > 0 {
		for k, v := range m.Project.FormDefaultText {
			st.Project.FormDefaultText = append(st.Project.FormDefaultText, &MgoMultiLang{Lang: k, Value: v})
		}
	}

	if m.Project != nil && m.Project.FirstPaymentAt != nil {
		st.Project.FirstPaymentAt, err = ptypes.Timestamp(m.Project.FirstPaymentAt)
		if err != nil {
			return nil, err
		}
	}

	if m.CanceledAt != nil {
		t, err := ptypes.Timestamp(m.CanceledAt)

		if err != nil {
			return nil, err
		}

		st.CanceledAt = t
	}

	if m.RefundedAt != nil {
		t, err := ptypes.Timestamp(m.RefundedAt)

		if err != nil {
			return nil, err
		}

		st.RefundedAt = t
	}

	if m.ParentPaymentAt != nil {
		t, err := ptypes.Timestamp(m.ParentPaymentAt)

		if err != nil {
			return nil, err
		}

		st.ParentPaymentAt = t
	}

	if len(m.Metadata) > 0 {
		st.MetadataValues = make([]string, 0)

		for _, val := range m.Metadata {
			st.MetadataValues = append(st.MetadataValues, val)
		}
	}

	return st, nil
}

func (o *orderMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	decoded := obj.(*MgoOrder)
	m := &billingpb.Order{}
	m.Id = decoded.Id.Hex()
	m.Uuid = decoded.Uuid
	m.Transaction = decoded.Transaction
	m.Object = decoded.Object
	m.Status = decoded.Status
	m.PrivateStatus = decoded.PrivateStatus
	m.Description = decoded.Description
	m.Canceled = decoded.Canceled
	m.Cancellation = decoded.Cancellation
	m.Refunded = decoded.Refunded
	m.ReceiptEmail = decoded.ReceiptEmail
	m.ReceiptPhone = decoded.ReceiptPhone
	m.ReceiptNumber = decoded.ReceiptNumber
	m.ReceiptUrl = decoded.ReceiptUrl
	m.AgreementVersion = decoded.AgreementVersion
	m.AgreementAccepted = decoded.AgreementAccepted
	m.NotifySale = decoded.NotifySale
	m.NotifySaleEmail = decoded.NotifySaleEmail
	m.Issuer = decoded.Issuer
	m.TotalPaymentAmount = decoded.TotalPaymentAmount
	m.Currency = decoded.Currency
	m.User = decoded.User
	m.BillingAddress = decoded.BillingAddress
	m.Tax = decoded.Tax
	m.PaymentMethod = getPaymentMethodOrder(decoded.PaymentMethod)
	m.Items = []*billingpb.OrderItem{}
	m.PlatformId = decoded.PlatformId
	m.ProductType = decoded.ProductType
	m.Keys = decoded.Keys
	m.IsKeyProductNotified = decoded.IsKeyProductNotified
	m.ReceiptId = decoded.ReceiptId
	m.IsBuyForVirtualCurrency = decoded.IsBuyForVirtualCurrency
	m.MccCode = decoded.MccCode
	m.OperatingCompanyId = decoded.OperatingCompanyId
	m.IsHighRisk = decoded.IsHighRisk
	m.ChargeCurrency = decoded.ChargeCurrency
	m.ChargeAmount = decoded.ChargeAmount
	m.PaymentIpCountry = decoded.PaymentIpCountry
	m.IsIpCountryMismatchBin = decoded.IsIpCountryMismatchBin
	m.BillingCountryChangedByUser = decoded.BillingCountryChangedByUser
	m.IsRefundAllowed = decoded.IsRefundAllowed
	m.FormMode = decoded.FormMode
	m.IsCurrencyPredefined = decoded.IsCurrencyPredefined
	m.Fee = decoded.Fee
	m.NetRevenue = decoded.NetRevenue
	m.RecurringPlanId = decoded.RecurringPlanId
	m.RecurringSubscriptionId = decoded.RecurringSubscriptionId

	if decoded.Refund != nil {
		m.Refund = &billingpb.OrderNotificationRefund{
			Amount:        decoded.Refund.Amount,
			Currency:      decoded.Refund.Currency,
			Reason:        decoded.Refund.Reason,
			Code:          decoded.Refund.Code,
			ReceiptNumber: decoded.Refund.ReceiptNumber,
			ReceiptUrl:    decoded.Refund.ReceiptUrl,
		}
	}
	m.Metadata = decoded.Metadata
	m.PrivateMetadata = decoded.PrivateMetadata
	m.Project = getOrderProject(decoded.Project)

	for _, v := range decoded.Items {
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
		item.CreatedAt, err = ptypes.TimestampProto(v.CreatedAt)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = ptypes.TimestampProto(v.UpdatedAt)
		if err != nil {
			return nil, err
		}
		m.Items = append(m.Items, item)
	}

	if decoded.Project != nil {
		nameLen := len(decoded.Project.Name)
		if nameLen > 0 {
			m.Project.Name = make(map[string]string, nameLen)

			for _, v := range decoded.Project.Name {
				m.Project.Name[v.Lang] = v.Value
			}
		}

		formDefaultTextLen := len(decoded.Project.FormDefaultText)
		if formDefaultTextLen > 0 {
			m.Project.FormDefaultText = make(map[string]string, formDefaultTextLen)

			for _, v := range decoded.Project.FormDefaultText {
				m.Project.FormDefaultText[v.Lang] = v.Value
			}
		}
	}

	m.ProjectParams = decoded.ProjectParams
	m.IsJsonRequest = decoded.IsJsonRequest
	m.OrderAmount = decoded.OrderAmount
	m.PaymentMethodPayerAccount = decoded.PaymentMethodPayerAccount
	m.PaymentMethodTxnParams = decoded.PaymentMethodTxnParams
	m.PaymentRequisites = decoded.PaymentRequisites
	m.UserAddressDataRequired = decoded.UserAddressDataRequired
	m.Products = decoded.Products
	m.IsNotificationsSent = decoded.IsNotificationsSent
	m.CountryRestriction = decoded.CountryRestriction
	m.ParentOrder = decoded.ParentOrder
	m.Type = decoded.Type
	m.IsVatDeduction = decoded.IsVatDeduction
	m.CountryCode = decoded.CountryCode
	m.VatPayer = decoded.VatPayer
	m.IsProduction = decoded.IsProduction
	m.MerchantInfo = decoded.MerchantInfo
	m.RoyaltyReportId = decoded.RoyaltyReportId

	m.PaymentMethodOrderClosedAt, err = ptypes.TimestampProto(decoded.PaymentMethodOrderClosedAt)
	if err != nil {
		return nil, err
	}

	m.ParentPaymentAt, err = ptypes.TimestampProto(decoded.ParentPaymentAt)

	if err != nil {
		return nil, err
	}

	m.ProjectLastRequestedAt, err = ptypes.TimestampProto(decoded.ProjectLastRequestedAt)
	if err != nil {
		return nil, err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)
	if err != nil {
		return nil, err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)
	if err != nil {
		return nil, err
	}
	m.CanceledAt, err = ptypes.TimestampProto(decoded.CanceledAt)
	if err != nil {
		return nil, err
	}

	m.RefundedAt, err = ptypes.TimestampProto(decoded.RefundedAt)
	if err != nil {
		return nil, err
	}

	m.ExpireDateToFormInput, err = ptypes.TimestampProto(decoded.ExpireDateToFormInput)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func NewOrderMapper() Mapper {
	return &orderMapper{}
}
