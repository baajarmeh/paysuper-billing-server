package billing

import (
	"errors"
	"fmt"
	"github.com/ProtocolONE/payone-repository/pkg/constant"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"time"
)

const (
	errorInvalidObjectId = "invalid bson object id"
	errorRequiredField   = "field \"%s\" is required to convert object %s"
)

type MgoVat struct {
	Id          bson.ObjectId `bson:"_id" json:"id"`
	Country     *Country      `bson:"country" json:"country"`
	Subdivision string        `bson:"subdivision_code" json:"subdivision_code,omitempty"`
	Vat         float64       `bson:"vat" json:"vat"`
	IsActive    bool          `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updated_at,omitempty"`
}

type MgoProject struct {
	Id                       bson.ObjectId                    `bson:"_id" json:"id"`
	CallbackCurrency         *Currency                        `bson:"callback_currency" json:"callback_currency"`
	CallbackProtocol         string                           `bson:"callback_protocol" json:"callback_protocol"`
	CreateInvoiceAllowedUrls []string                         `bson:"create_invoice_allowed_urls" json:"create_invoice_allowed_urls"`
	Merchant                 *MgoMerchant                     `bson:"merchant" json:"-"`
	AllowDynamicNotifyUrls   bool                             `bson:"is_allow_dynamic_notify_urls" json:"allow_dynamic_notify_urls"`
	AllowDynamicRedirectUrls bool                             `bson:"is_allow_dynamic_redirect_urls" json:"allow_dynamic_redirect_urls"`
	LimitsCurrency           *Currency                        `bson:"limits_currency" json:"limits_currency"`
	MaxPaymentAmount         float64                          `bson:"max_payment_amount" json:"max_payment_amount"`
	MinPaymentAmount         float64                          `bson:"min_payment_amount" json:"min_payment_amount"`
	Name                     string                           `bson:"name" json:"name"`
	NotifyEmails             []string                         `bson:"notify_emails" json:"notify_emails"`
	OnlyFixedAmounts         bool                             `bson:"only_fixed_amounts" json:"only_fixed_amounts"`
	SecretKey                string                           `bson:"secret_key" json:"secret_key"`
	SendNotifyEmail          bool                             `bson:"send_notify_email" json:"send_notify_email"`
	UrlCheckAccount          string                           `bson:"url_check_account" json:"url_check_account"`
	UrlProcessPayment        string                           `bson:"url_process_payment" json:"url_process_payment"`
	UrlRedirectFail          string                           `bson:"url_redirect_fail" json:"url_redirect_fail"`
	UrlRedirectSuccess       string                           `bson:"url_redirect_success" json:"url_redirect_success"`
	IsActive                 bool                             `bson:"is_active" json:"is_active"`
	CreatedAt                time.Time                        `bson:"created_at" json:"created_at"`
	UpdatedAt                time.Time                        `bson:"updated_at" json:"-"`
	FixedPackage             map[string][]*FixedPackage       `bson:"fixed_package" json:"fixed_package,omitempty"`
	PaymentMethods           map[string]*ProjectPaymentMethod `bson:"payment_methods" json:"payment_methods,omitempty"`
}

type MgoMerchant struct {
	Id                        bson.ObjectId `bson:"_id"`
	ExternalId                string        `bson:"external_id"`
	Email                     string        `bson:"email"`
	Name                      string        `bson:"name"`
	Country                   *Country      `bson:"country"`
	AccountingPeriod          string        `bson:"accounting_period"`
	Currency                  *Currency     `bson:"currency"`
	IsVatEnabled              bool          `bson:"is_vat_enabled"`
	IsCommissionToUserEnabled bool          `bson:"is_commission_to_user_enabled"`
	Status                    int32         `bson:"status"`
	CreatedAt                 time.Time     `bson:"created_at"`
	UpdatedAt                 time.Time     `bson:"updated_at"`
	FirstPaymentAt            time.Time     `bson:"first_payment_at"`
}

type MgoCurrencyRate struct {
	Id           bson.ObjectId `bson:"_id"`
	CurrencyFrom int32         `bson:"currency_from"`
	CurrencyTo   int32         `bson:"currency_to"`
	Rate         float64       `bson:"rate"`
	Date         time.Time     `bson:"date"`
	IsActive     bool          `bson:"is_active"`
	CreatedAt    time.Time     `bson:"created_at"`
}

type MgoCommission struct {
	Id struct {
		PaymentMethodId bson.ObjectId `bson:"pm_id"`
		ProjectId       bson.ObjectId `bson:"project_id"`
	} `bson:"_id"`
	PaymentMethodCommission float64   `bson:"pm_commission"`
	PspCommission           float64   `bson:"psp_commission"`
	ToUserCommission        float64   `bson:"total_commission_to_user"`
	StartDate               time.Time `bson:"start_date"`
}

type MgoCommissionBilling struct {
	Id                      bson.ObjectId `bson:"_id"`
	PaymentMethodId         bson.ObjectId `bson:"pm_id"`
	ProjectId               bson.ObjectId `bson:"project_id"`
	PaymentMethodCommission float64       `bson:"pm_commission"`
	PspCommission           float64       `bson:"psp_commission"`
	TotalCommissionToUser   float64       `bson:"total_commission_to_user"`
	StartDate               time.Time     `bson:"start_date"`
	CreatedAt               time.Time     `bson:"created_at"`
	UpdatedAt               time.Time     `bson:"updated_at"`
}

type MgoOrderProject struct {
	Id                bson.ObjectId `bson:"_id" `
	Name              string        `bson:"name"`
	UrlSuccess        string        `bson:"url_success"`
	UrlFail           string        `bson:"url_fail"`
	NotifyEmails      []string      `bson:"notify_emails"`
	SecretKey         string        `bson:"secret_key"`
	SendNotifyEmail   bool          `bson:"send_notify_email"`
	UrlCheckAccount   string        `bson:"url_check_account"`
	UrlProcessPayment string        `bson:"url_process_payment"`
	CallbackProtocol  string        `bson:"callback_protocol"`
	Merchant          *MgoMerchant  `bson:"merchant"`
}

type MgoOrderPaymentMethod struct {
	Id            bson.ObjectId        `bson:"_id"`
	Name          string               `bson:"name"`
	Params        *PaymentMethodParams `bson:"params"`
	PaymentSystem *PaymentSystem       `bson:"payment_system"`
	Group         string               `bson:"group_alias"`
}

type MgoOrder struct {
	Id                                      bson.ObjectId          `bson:"_id"`
	Project                                 *MgoOrderProject       `bson:"project"`
	ProjectOrderId                          string                 `bson:"project_order_id"`
	ProjectAccount                          string                 `bson:"project_account"`
	Description                             string                 `bson:"description"`
	ProjectIncomeAmount                     float64                `bson:"project_income_amount"`
	ProjectIncomeCurrency                   *Currency              `bson:"project_income_currency"`
	ProjectOutcomeAmount                    float64                `bson:"project_outcome_amount"`
	ProjectOutcomeCurrency                  *Currency              `bson:"project_outcome_currency"`
	ProjectLastRequestedAt                  time.Time              `bson:"project_last_requested_at"`
	ProjectParams                           map[string]string      `bson:"project_params"`
	PayerData                               *PayerData             `bson:"payer_data"`
	PaymentMethod                           *MgoOrderPaymentMethod `bson:"payment_method"`
	PaymentMethodTerminalId                 string                 `bson:"pm_terminal_id"`
	PaymentMethodOrderId                    string                 `bson:"pm_order_id"`
	PaymentMethodOutcomeAmount              float64                `bson:"pm_outcome_amount"`
	PaymentMethodOutcomeCurrency            *Currency              `bson:"pm_outcome_currency"`
	PaymentMethodIncomeAmount               float64                `bson:"pm_income_amount"`
	PaymentMethodIncomeCurrency             *Currency              `bson:"pm_income_currency"`
	PaymentMethodOrderClosedAt              time.Time              `bson:"pm_order_close_date"`
	Status                                  int32                  `bson:"status"`
	IsJsonRequest                           bool                   `bson:"created_by_json"`
	AmountInPspAccountingCurrency           float64                `bson:"amount_psp_ac"`
	AmountInMerchantAccountingCurrency      float64                `bson:"amount_in_merchant_ac"`
	AmountOutMerchantAccountingCurrency     float64                `bson:"amount_out_merchant_ac"`
	AmountInPaymentSystemAccountingCurrency float64                `bson:"amount_ps_ac"`
	PaymentMethodPayerAccount               string                 `bson:"pm_account"`
	PaymentMethodTxnParams                  map[string]string      `bson:"pm_txn_params"`
	FixedPackage                            *FixedPackage          `bson:"fixed_package"`
	PaymentRequisites                       map[string]string      `bson:"payment_requisites"`
	PspFeeAmount                            *OrderFeePsp           `bson:"psp_fee_amount"`
	ProjectFeeAmount                        *OrderFee              `bson:"project_fee_amount"`
	ToPayerFeeAmount                        *OrderFee              `bson:"to_payer_fee_amount"`
	VatAmount                               *OrderFee              `bson:"vat_amount"`
	PaymentSystemFeeAmount                  *OrderFeePaymentSystem `bson:"ps_fee_amount"`
	UrlSuccess                              string                 `bson:"url_success"`
	UrlFail                                 string                 `bson:"url_fail"`
	CreatedAt                               time.Time              `bson:"created_at"`
	UpdatedAt                               time.Time              `bson:"updated_at"`
}

type MgoPaymentSystem struct {
	Id                 bson.ObjectId `bson:"_id"`
	Name               string        `bson:"name"`
	Country            *Country      `bson:"country"`
	AccountingCurrency *Currency     `bson:"accounting_currency"`
	AccountingPeriod   string        `bson:"accounting_period"`
	IsActive           bool          `bson:"is_active"`
	CreatedAt          time.Time     `bson:"created_at"`
	UpdatedAt          time.Time     `bson:"updated_at"`
}

type MgoPaymentMethod struct {
	Id               bson.ObjectId        `bson:"_id"`
	Name             string               `bson:"name"`
	Group            string               `bson:"group_alias"`
	Currency         *Currency            `bson:"currency"`
	MinPaymentAmount float64              `bson:"min_payment_amount"`
	MaxPaymentAmount float64              `bson:"max_payment_amount"`
	Params           *PaymentMethodParams `bson:"params"`
	Icon             string               `bson:"icon"`
	IsActive         bool                 `bson:"is_active"`
	CreatedAt        time.Time            `bson:"created_at"`
	UpdatedAt        time.Time            `bson:"updated_at"`
	PaymentSystem    *MgoPaymentSystem    `bson:"payment_system"`
	Currencies       []int32              `bson:"currencies"`
	Type             string               `bson:"type"`
	AccountRegexp    string               `bson:"account_regexp"`
}

func (m *Vat) GetBSON() (interface{}, error) {
	st := &MgoVat{
		Country:     m.Country,
		Subdivision: m.Subdivision,
		Vat:         m.Vat,
		IsActive:    m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
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

	return st, nil
}

func (m *Vat) SetBSON(raw bson.Raw) error {
	decoded := new(MgoVat)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Country = decoded.Country
	m.Subdivision = decoded.Subdivision
	m.Vat = decoded.Vat
	m.IsActive = decoded.IsActive

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *Project) GetBSON() (interface{}, error) {
	st := &MgoProject{
		CallbackCurrency:         m.CallbackCurrency,
		CreateInvoiceAllowedUrls: m.CreateInvoiceAllowedUrls,
		AllowDynamicNotifyUrls:   m.AllowDynamicNotifyUrls,
		AllowDynamicRedirectUrls: m.AllowDynamicRedirectUrls,
		LimitsCurrency:           m.LimitsCurrency,
		MaxPaymentAmount:         m.MaxPaymentAmount,
		MinPaymentAmount:         m.MinPaymentAmount,
		Name:                     m.Name,
		NotifyEmails:             m.NotifyEmails,
		OnlyFixedAmounts:         m.OnlyFixedAmounts,
		SecretKey:                m.SecretKey,
		SendNotifyEmail:          m.SendNotifyEmail,
		UrlCheckAccount:          m.UrlCheckAccount,
		UrlProcessPayment:        m.UrlProcessPayment,
		UrlRedirectFail:          m.UrlRedirectFail,
		UrlRedirectSuccess:       m.UrlRedirectSuccess,
		IsActive:                 m.IsActive,
		PaymentMethods:           m.PaymentMethods,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if len(m.CallbackProtocol) <= 0 {
		st.CallbackProtocol = "default"
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

	if len(m.FixedPackage) > 0 {
		fps := make(map[string][]*FixedPackage)

		for c, ofp := range m.FixedPackage {
			fps[c] = ofp.FixedPackage
		}

		st.FixedPackage = fps
	}

	if m.Merchant != nil {
		st.Merchant = &MgoMerchant{
			Id:                        bson.ObjectIdHex(m.Merchant.Id),
			ExternalId:                m.Merchant.ExternalId,
			Email:                     m.Merchant.Email,
			Name:                      m.Merchant.Name,
			Country:                   m.Merchant.Country,
			AccountingPeriod:          m.Merchant.AccountingPeriod,
			Currency:                  m.Merchant.Currency,
			IsVatEnabled:              m.Merchant.IsVatEnabled,
			IsCommissionToUserEnabled: m.Merchant.IsCommissionToUserEnabled,
			Status:                    m.Merchant.Status,
		}

		if m.Merchant.CreatedAt != nil {
			t, err := ptypes.Timestamp(m.Merchant.CreatedAt)

			if err != nil {
				return nil, err
			}

			st.Merchant.CreatedAt = t
		} else {
			st.Merchant.CreatedAt = time.Now()
		}

		if m.Merchant.UpdatedAt != nil {
			t, err := ptypes.Timestamp(m.Merchant.UpdatedAt)

			if err != nil {
				return nil, err
			}

			st.Merchant.UpdatedAt = t
		} else {
			st.Merchant.UpdatedAt = time.Now()
		}

		if m.Merchant.FirstPaymentAt != nil {
			t, err := ptypes.Timestamp(m.Merchant.FirstPaymentAt)

			if err != nil {
				return nil, err
			}

			st.Merchant.FirstPaymentAt = t
		} else {
			st.Merchant.FirstPaymentAt = time.Now()
		}
	}

	return st, nil
}

func (m *Project) SetBSON(raw bson.Raw) error {
	decoded := new(MgoProject)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.CallbackCurrency = decoded.CallbackCurrency
	m.CallbackProtocol = decoded.CallbackProtocol
	m.CreateInvoiceAllowedUrls = decoded.CreateInvoiceAllowedUrls

	if decoded.Merchant != nil {
		m.Merchant = &Merchant{
			Id:                        decoded.Merchant.Id.Hex(),
			ExternalId:                decoded.Merchant.ExternalId,
			Email:                     decoded.Merchant.Email,
			Name:                      decoded.Merchant.Name,
			Country:                   decoded.Merchant.Country,
			AccountingPeriod:          decoded.Merchant.AccountingPeriod,
			Currency:                  decoded.Merchant.Currency,
			IsVatEnabled:              decoded.Merchant.IsVatEnabled,
			IsCommissionToUserEnabled: decoded.Merchant.IsCommissionToUserEnabled,
			Status:                    decoded.Merchant.Status,
		}

		m.Merchant.CreatedAt, err = ptypes.TimestampProto(decoded.Merchant.CreatedAt)

		if err != nil {
			return err
		}

		m.Merchant.UpdatedAt, err = ptypes.TimestampProto(decoded.Merchant.UpdatedAt)

		if err != nil {
			return err
		}

		m.Merchant.FirstPaymentAt, err = ptypes.TimestampProto(decoded.Merchant.FirstPaymentAt)

		if err != nil {
			return err
		}
	}

	m.AllowDynamicNotifyUrls = decoded.AllowDynamicNotifyUrls
	m.AllowDynamicRedirectUrls = decoded.AllowDynamicRedirectUrls
	m.LimitsCurrency = decoded.LimitsCurrency
	m.MaxPaymentAmount = decoded.MaxPaymentAmount
	m.MinPaymentAmount = decoded.MinPaymentAmount
	m.Name = decoded.Name
	m.NotifyEmails = decoded.NotifyEmails
	m.OnlyFixedAmounts = decoded.OnlyFixedAmounts
	m.SecretKey = decoded.SecretKey
	m.SendNotifyEmail = decoded.SendNotifyEmail
	m.UrlCheckAccount = decoded.UrlCheckAccount
	m.UrlProcessPayment = decoded.UrlProcessPayment
	m.UrlRedirectFail = decoded.UrlRedirectFail
	m.UrlRedirectSuccess = decoded.UrlRedirectSuccess
	m.PaymentMethods = decoded.PaymentMethods
	m.IsActive = decoded.IsActive

	if len(decoded.FixedPackage) > 0 {
		fps := make(map[string]*FixedPackages)

		for c, fp := range decoded.FixedPackage {
			fps[c] = &FixedPackages{FixedPackage: fp}
		}

		m.FixedPackage = fps
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *CurrencyRate) GetBSON() (interface{}, error) {
	st := &MgoCurrencyRate{
		CurrencyFrom: m.CurrencyFrom,
		CurrencyTo:   m.CurrencyTo,
		Rate:         m.Rate,
		IsActive:     m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.Date == nil {
		return nil, fmt.Errorf(errorRequiredField, "Date", "CurrencyRate")
	}

	t, err := ptypes.Timestamp(m.Date)

	if err != nil {
		return nil, err
	}

	st.Date = t

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	return st, nil
}

func (m *CurrencyRate) SetBSON(raw bson.Raw) error {
	decoded := new(MgoCurrencyRate)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.CurrencyFrom = decoded.CurrencyFrom
	m.CurrencyTo = decoded.CurrencyTo
	m.Rate = decoded.Rate
	m.IsActive = decoded.IsActive

	m.Date, err = ptypes.TimestampProto(decoded.Date)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	return err
}

func (m *Commission) GetBSON() (interface{}, error) {
	st := &MgoCommissionBilling{
		PaymentMethodId:         bson.ObjectIdHex(m.PaymentMethodId),
		ProjectId:               bson.ObjectIdHex(m.ProjectId),
		PaymentMethodCommission: m.PaymentMethodCommission,
		PspCommission:           m.PspCommission,
		TotalCommissionToUser:   m.TotalCommissionToUser,
	}

	t, err := ptypes.Timestamp(m.StartDate)

	if err != nil {
		return nil, err
	}

	st.StartDate = t

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
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

	return st, nil
}

func (m *Commission) SetBSON(raw bson.Raw) error {
	decoded := new(MgoCommissionBilling)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.PaymentMethodId = decoded.PaymentMethodId.Hex()
	m.ProjectId = decoded.ProjectId.Hex()
	m.PaymentMethodCommission = decoded.PaymentMethodCommission
	m.PspCommission = decoded.PspCommission
	m.TotalCommissionToUser = decoded.TotalCommissionToUser

	m.StartDate, err = ptypes.TimestampProto(decoded.StartDate)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	return err
}

func (m *Order) GetBSON() (interface{}, error) {
	st := &MgoOrder{
		Project: &MgoOrderProject{
			Id:                bson.ObjectIdHex(m.Project.Id),
			Name:              m.Project.Name,
			UrlSuccess:        m.Project.UrlSuccess,
			UrlFail:           m.Project.UrlFail,
			NotifyEmails:      m.Project.NotifyEmails,
			SendNotifyEmail:   m.Project.SendNotifyEmail,
			SecretKey:         m.Project.SecretKey,
			UrlCheckAccount:   m.Project.UrlCheckAccount,
			UrlProcessPayment: m.Project.UrlProcessPayment,
			CallbackProtocol:  m.Project.CallbackProtocol,
			Merchant: &MgoMerchant{
				Id:                        bson.ObjectIdHex(m.Project.Merchant.Id),
				ExternalId:                m.Project.Merchant.ExternalId,
				Email:                     m.Project.Merchant.Email,
				Name:                      m.Project.Merchant.Name,
				Country:                   m.Project.Merchant.Country,
				AccountingPeriod:          m.Project.Merchant.AccountingPeriod,
				Currency:                  m.Project.Merchant.Currency,
				IsVatEnabled:              m.Project.Merchant.IsVatEnabled,
				IsCommissionToUserEnabled: m.Project.Merchant.IsCommissionToUserEnabled,
				Status:                    m.Project.Merchant.Status,
			},
		},
		ProjectOrderId:                          m.ProjectOrderId,
		ProjectAccount:                          m.ProjectAccount,
		Description:                             m.Description,
		ProjectIncomeAmount:                     m.ProjectIncomeAmount,
		ProjectIncomeCurrency:                   m.ProjectIncomeCurrency,
		ProjectOutcomeAmount:                    m.ProjectOutcomeAmount,
		ProjectOutcomeCurrency:                  m.ProjectOutcomeCurrency,
		ProjectParams:                           m.ProjectParams,
		PayerData:                               m.PayerData,
		PaymentMethodOrderId:                    m.PaymentMethodOrderId,
		PaymentMethodOutcomeAmount:              m.PaymentMethodOutcomeAmount,
		PaymentMethodOutcomeCurrency:            m.PaymentMethodOutcomeCurrency,
		PaymentMethodIncomeAmount:               m.PaymentMethodIncomeAmount,
		PaymentMethodIncomeCurrency:             m.PaymentMethodIncomeCurrency,
		Status:                                  m.Status,
		IsJsonRequest:                           m.IsJsonRequest,
		AmountInPspAccountingCurrency:           m.AmountInPspAccountingCurrency,
		AmountInMerchantAccountingCurrency:      m.AmountInMerchantAccountingCurrency,
		AmountOutMerchantAccountingCurrency:     m.AmountOutMerchantAccountingCurrency,
		AmountInPaymentSystemAccountingCurrency: m.AmountInPaymentSystemAccountingCurrency,
		PaymentMethodPayerAccount:               m.PaymentMethodPayerAccount,
		PaymentMethodTxnParams:                  m.PaymentMethodTxnParams,
		FixedPackage:                            m.FixedPackage,
		PaymentRequisites:                       m.PaymentRequisites,
		PspFeeAmount:                            m.PspFeeAmount,
		ProjectFeeAmount:                        m.ProjectFeeAmount,
		ToPayerFeeAmount:                        m.ToPayerFeeAmount,
		VatAmount:                               m.VatAmount,
		PaymentSystemFeeAmount:                  m.PaymentSystemFeeAmount,
	}

	if m.PaymentMethod != nil {
		st.PaymentMethod = &MgoOrderPaymentMethod{
			Id:            bson.ObjectIdHex(m.PaymentMethod.Id),
			Name:          m.PaymentMethod.Name,
			Params:        m.PaymentMethod.Params,
			PaymentSystem: m.PaymentMethod.PaymentSystem,
			Group:         m.PaymentMethod.Group,
		}
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
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

	if m.Project.Merchant.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.Project.Merchant.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.Project.Merchant.CreatedAt = t
	} else {
		st.Project.Merchant.CreatedAt = time.Now()
	}

	if m.Project.Merchant.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.Project.Merchant.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.Project.Merchant.UpdatedAt = t
	} else {
		st.Project.Merchant.UpdatedAt = time.Now()
	}

	if m.Project.Merchant.FirstPaymentAt != nil {
		t, err := ptypes.Timestamp(m.Project.Merchant.FirstPaymentAt)

		if err != nil {
			return nil, err
		}

		st.Project.Merchant.FirstPaymentAt = t
	}

	return st, nil
}

func (m *Order) SetBSON(raw bson.Raw) error {
	decoded := new(MgoOrder)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Project = &ProjectOrder{
		Id:                decoded.Project.Id.Hex(),
		Name:              decoded.Project.Name,
		UrlSuccess:        decoded.Project.UrlSuccess,
		UrlFail:           decoded.Project.UrlFail,
		NotifyEmails:      decoded.Project.NotifyEmails,
		SendNotifyEmail:   decoded.Project.SendNotifyEmail,
		SecretKey:         decoded.Project.SecretKey,
		UrlCheckAccount:   decoded.Project.UrlCheckAccount,
		UrlProcessPayment: decoded.Project.UrlProcessPayment,
		CallbackProtocol:  decoded.Project.CallbackProtocol,
		Merchant: &Merchant{
			Id:                        decoded.Project.Merchant.Id.Hex(),
			ExternalId:                decoded.Project.Merchant.ExternalId,
			Email:                     decoded.Project.Merchant.Email,
			Name:                      decoded.Project.Merchant.Name,
			Country:                   decoded.Project.Merchant.Country,
			AccountingPeriod:          decoded.Project.Merchant.AccountingPeriod,
			Currency:                  decoded.Project.Merchant.Currency,
			IsVatEnabled:              decoded.Project.Merchant.IsVatEnabled,
			IsCommissionToUserEnabled: decoded.Project.Merchant.IsCommissionToUserEnabled,
			Status:                    decoded.Project.Merchant.Status,
		},
	}

	m.ProjectOrderId = decoded.ProjectOrderId
	m.ProjectAccount = decoded.ProjectAccount
	m.Description = decoded.Description
	m.ProjectIncomeAmount = decoded.ProjectIncomeAmount
	m.ProjectIncomeCurrency = decoded.ProjectIncomeCurrency
	m.ProjectOutcomeAmount = decoded.ProjectOutcomeAmount
	m.ProjectOutcomeCurrency = decoded.ProjectOutcomeCurrency
	m.ProjectParams = decoded.ProjectParams
	m.PayerData = decoded.PayerData

	if decoded.PaymentMethod != nil {
		m.PaymentMethod = &PaymentMethodOrder{
			Id:            decoded.PaymentMethod.Id.Hex(),
			Name:          decoded.PaymentMethod.Name,
			Params:        decoded.PaymentMethod.Params,
			PaymentSystem: decoded.PaymentMethod.PaymentSystem,
			Group:         decoded.PaymentMethod.Group,
		}
	}

	m.PaymentMethodOrderId = decoded.PaymentMethodOrderId
	m.PaymentMethodOutcomeAmount = decoded.PaymentMethodOutcomeAmount
	m.PaymentMethodOutcomeCurrency = decoded.PaymentMethodOutcomeCurrency
	m.PaymentMethodIncomeAmount = decoded.PaymentMethodIncomeAmount
	m.PaymentMethodIncomeCurrency = decoded.PaymentMethodIncomeCurrency
	m.Status = decoded.Status
	m.IsJsonRequest = decoded.IsJsonRequest
	m.AmountInPspAccountingCurrency = decoded.AmountInPspAccountingCurrency
	m.AmountInMerchantAccountingCurrency = decoded.AmountInMerchantAccountingCurrency
	m.AmountOutMerchantAccountingCurrency = decoded.AmountOutMerchantAccountingCurrency
	m.AmountInPaymentSystemAccountingCurrency = decoded.AmountInPaymentSystemAccountingCurrency
	m.PaymentMethodPayerAccount = decoded.PaymentMethodPayerAccount
	m.PaymentMethodTxnParams = decoded.PaymentMethodTxnParams
	m.FixedPackage = decoded.FixedPackage
	m.PaymentRequisites = decoded.PaymentRequisites
	m.PspFeeAmount = decoded.PspFeeAmount
	m.ProjectFeeAmount = decoded.ProjectFeeAmount
	m.ToPayerFeeAmount = decoded.ToPayerFeeAmount
	m.VatAmount = decoded.VatAmount
	m.PaymentSystemFeeAmount = decoded.PaymentSystemFeeAmount

	m.PaymentMethodOrderClosedAt, err = ptypes.TimestampProto(decoded.PaymentMethodOrderClosedAt)

	if err != nil {
		return err
	}

	m.ProjectLastRequestedAt, err = ptypes.TimestampProto(decoded.ProjectLastRequestedAt)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	m.Project.Merchant.CreatedAt, err = ptypes.TimestampProto(decoded.Project.Merchant.CreatedAt)

	if err != nil {
		return err
	}

	m.Project.Merchant.UpdatedAt, err = ptypes.TimestampProto(decoded.Project.Merchant.UpdatedAt)

	if err != nil {
		return err
	}

	m.Project.Merchant.FirstPaymentAt, err = ptypes.TimestampProto(decoded.Project.Merchant.FirstPaymentAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *PaymentMethod) GetBSON() (interface{}, error) {
	st := &MgoPaymentMethod{
		Name:             m.Name,
		Group:            m.Group,
		Currency:         m.Currency,
		MinPaymentAmount: m.MinPaymentAmount,
		MaxPaymentAmount: m.MaxPaymentAmount,
		Params:           m.Params,
		Icon:             m.Icon,
		Currencies:       m.Currencies,
		Type:             m.Type,
		AccountRegexp:    m.AccountRegexp,
		IsActive:         m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.PaymentSystem != nil {
		st.PaymentSystem = &MgoPaymentSystem{
			Id:                 bson.ObjectIdHex(m.PaymentSystem.Id),
			Name:               m.PaymentSystem.Name,
			Country:            m.PaymentSystem.Country,
			AccountingCurrency: m.PaymentSystem.AccountingCurrency,
			AccountingPeriod:   m.PaymentSystem.AccountingPeriod,
			IsActive:           m.PaymentSystem.IsActive,
		}

		if m.PaymentSystem.CreatedAt != nil {
			t, err := ptypes.Timestamp(m.PaymentSystem.CreatedAt)

			if err != nil {
				return nil, err
			}

			st.PaymentSystem.CreatedAt = t
		} else {
			st.PaymentSystem.CreatedAt = time.Now()
		}

		if m.PaymentSystem.UpdatedAt != nil {
			t, err := ptypes.Timestamp(m.PaymentSystem.UpdatedAt)

			if err != nil {
				return nil, err
			}

			st.PaymentSystem.UpdatedAt = t
		} else {
			st.PaymentSystem.UpdatedAt = time.Now()
		}
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

	return st, nil
}

func (m *PaymentMethod) SetBSON(raw bson.Raw) error {
	decoded := new(MgoPaymentMethod)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Name = decoded.Name
	m.Group = decoded.Group
	m.Currency = decoded.Currency
	m.Currencies = decoded.Currencies
	m.MinPaymentAmount = decoded.MinPaymentAmount
	m.MaxPaymentAmount = decoded.MaxPaymentAmount
	m.Params = decoded.Params
	m.Icon = decoded.Icon
	m.Type = decoded.Type
	m.AccountRegexp = decoded.AccountRegexp
	m.IsActive = decoded.IsActive

	if decoded.PaymentSystem != nil {
		m.PaymentSystem = &PaymentSystem{
			Id:                 decoded.PaymentSystem.Id.Hex(),
			Name:               decoded.PaymentSystem.Name,
			Country:            decoded.PaymentSystem.Country,
			AccountingCurrency: decoded.PaymentSystem.AccountingCurrency,
			AccountingPeriod:   decoded.PaymentSystem.AccountingPeriod,
			IsActive:           decoded.PaymentSystem.IsActive,
		}

		m.PaymentSystem.CreatedAt, err = ptypes.TimestampProto(decoded.PaymentSystem.CreatedAt)

		if err != nil {
			return err
		}

		m.PaymentSystem.UpdatedAt, err = ptypes.TimestampProto(decoded.PaymentSystem.UpdatedAt)

		if err != nil {
			return err
		}
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *PaymentSystem) GetBSON() (interface{}, error) {
	st := &MgoPaymentSystem{
		Name:               m.Name,
		Country:            m.Country,
		AccountingCurrency: m.AccountingCurrency,
		AccountingPeriod:   m.AccountingPeriod,
		IsActive:           m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
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

	return st, nil
}

func (m *PaymentSystem) SetBSON(raw bson.Raw) error {
	decoded := new(MgoPaymentSystem)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Name = decoded.Name
	m.Country = decoded.Country
	m.AccountingCurrency = decoded.AccountingCurrency
	m.AccountingPeriod = decoded.AccountingPeriod
	m.IsActive = decoded.IsActive

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *Merchant) GetBSON() (interface{}, error) {
	st := &MgoMerchant{
		ExternalId:                m.ExternalId,
		Email:                     m.Email,
		Name:                      m.Name,
		Country:                   m.Country,
		AccountingPeriod:          m.AccountingPeriod,
		Currency:                  m.Currency,
		IsVatEnabled:              m.IsVatEnabled,
		IsCommissionToUserEnabled: m.IsCommissionToUserEnabled,
		Status:                    m.Status,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.FirstPaymentAt != nil {
		t, err := ptypes.Timestamp(m.FirstPaymentAt)

		if err != nil {
			return nil, err
		}

		st.FirstPaymentAt = t
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

	return st, nil
}

func (m *Merchant) SetBSON(raw bson.Raw) error {
	decoded := new(MgoMerchant)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.ExternalId = decoded.ExternalId
	m.Email = decoded.Email
	m.Name = decoded.Name
	m.Country = decoded.Country
	m.AccountingPeriod = decoded.AccountingPeriod
	m.Currency = decoded.Currency
	m.IsVatEnabled = decoded.IsVatEnabled
	m.IsCommissionToUserEnabled = decoded.IsCommissionToUserEnabled
	m.Status = decoded.Status

	m.FirstPaymentAt, err = ptypes.TimestampProto(decoded.FirstPaymentAt)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *PaymentFormPaymentMethod) IsBankCard() bool {
	return m.Group == constant.PaymentSystemGroupAliasBankCard
}

func (m *PaymentMethod) IsBankCard() bool {
	return m.Group == constant.PaymentSystemGroupAliasBankCard
}

func (m *Order) HasEndedStatus() bool {
	return m.Status == constant.OrderStatusPaymentSystemReject || m.Status == constant.OrderStatusProjectComplete ||
		m.Status == constant.OrderStatusProjectReject || m.Status == constant.OrderStatusRefund ||
		m.Status == constant.OrderStatusChargeback
}