package service

import (
	"context"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"math/rand"
	"net"
	"strings"
	"time"
)

const (
	tokenStorageMask   = "paysuper:token:%s"
	tokenLetterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	tokenLetterIdxBits = uint(6)
	tokenLetterIdxMask = uint64(1<<tokenLetterIdxBits - 1)
	tokenLetterIdxMax  = 63 / tokenLetterIdxBits
)

var (
	tokenErrorUnknown              = newBillingServerErrorMsg("tk000001", "unknown token error")
	customerNotFound               = newBillingServerErrorMsg("tk000002", "customer by specified data not found")
	tokenErrorNotFound             = newBillingServerErrorMsg("tk000003", "token not found")
	tokenErrorUserIdentityRequired = newBillingServerErrorMsg("tk000004", "request must contain one or more parameters with user information")

	tokenErrorSettingsTypeRequired                            = newBillingServerErrorMsg("tk000005", `field settings.type is required`)
	tokenErrorSettingsSimpleCheckoutParamsRequired            = newBillingServerErrorMsg("tk000006", `fields settings.amount and settings.currency is required for creating payment token with type "simple"`)
	tokenErrorSettingsProductAndKeyProductIdsParamsRequired   = newBillingServerErrorMsg("tk000007", `field settings.product_ids is required for creating payment token with type "product" or "key"`)
	tokenErrorSettingsAmountAndCurrencyParamNotAllowedForType = newBillingServerErrorMsg("tk000008", `fields settings.amount and settings.currency not allowed for creating payment token with types "product" or "key"`)
	tokenErrorSettingsProductIdsParamNotAllowedForType        = newBillingServerErrorMsg("tk000009", `fields settings.product_ids not allowed for creating payment token with type "simple"`)

	tokenRandSource = rand.NewSource(time.Now().UnixNano())
)

type Token struct {
	CustomerId string                   `json:"customer_id"`
	User       *billingpb.TokenUser     `json:"user"`
	Settings   *billingpb.TokenSettings `json:"settings"`
}

type tokenRepository struct {
	token   *Token
	service *Service
}

type BrowserCookieCustomer struct {
	CustomerId        string    `json:"customer_id"`
	VirtualCustomerId string    `json:"virtual_customer_id"`
	Ip                string    `json:"ip"`
	IpCountry         string    `json:"ip_country"`
	SelectedCountry   string    `json:"selected_country"`
	UserAgent         string    `json:"user_agent"`
	AcceptLanguage    string    `json:"accept_language"`
	SessionCount      int32     `json:"session_count"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (s *Service) CreateToken(
	ctx context.Context,
	req *billingpb.TokenRequest,
	rsp *billingpb.TokenResponse,
) error {
	identityExist := req.User.Id != "" || (req.User.Email != nil && req.User.Email.Value != "") ||
		(req.User.Phone != nil && req.User.Phone.Value != "")

	if identityExist == false {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = tokenErrorUserIdentityRequired

		return nil
	}

	processor := &OrderCreateRequestProcessor{
		Service: s,
		request: &billingpb.OrderCreateRequest{
			ProjectId:  req.Settings.ProjectId,
			Amount:     req.Settings.Amount,
			Currency:   req.Settings.Currency,
			Products:   req.Settings.ProductsIds,
			PlatformId: req.Settings.PlatformId,
		},
		checked: &orderCreateRequestProcessorChecked{
			user: &billingpb.OrderUser{},
		},
		ctx: ctx,
	}

	err := processor.processProject()

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = err.(*billingpb.ResponseErrorMessage)
		return nil
	}

	err = processor.processMerchant()

	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = err.(*billingpb.ResponseErrorMessage)
		return nil
	}

	if req.Settings.Type == pkg.OrderType_product || req.Settings.Type == pkg.OrderType_key {
		if req.Settings.Amount > 0 || req.Settings.Currency != "" {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = tokenErrorSettingsAmountAndCurrencyParamNotAllowedForType
			return nil
		}

		if len(req.Settings.ProductsIds) <= 0 {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = tokenErrorSettingsProductAndKeyProductIdsParamsRequired
			return nil
		}
	}

	if req.User != nil {
		if req.User.Address != nil && req.User.Address.Country != "" {
			processor.checked.user.Address = req.User.Address
		} else {
			if req.User.Ip != nil {
				address, err := s.getAddressByIp(ctx, req.User.Ip.Value)
				if err != nil {
					zap.L().Error(pkg.MethodFinishedWithError, zap.Error(err))
					if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
						rsp.Status = billingpb.ResponseStatusBadData
						rsp.Message = e
						return nil
					}
					return err
				}
				processor.checked.user.Address = address
			}
		}
	}

	err = processor.processCurrency(req.Settings.Type)
	if err != nil {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = err.(*billingpb.ResponseErrorMessage)
		return nil
	}

	switch req.Settings.Type {
	case pkg.OrderType_simple:
		if len(req.Settings.ProductsIds) > 0 {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = tokenErrorSettingsProductIdsParamNotAllowedForType
			return nil
		}

		if req.Settings.Amount <= 0 || req.Settings.Currency == "" {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = tokenErrorSettingsSimpleCheckoutParamsRequired
			return nil
		}

		processor.processAmount()
		err = processor.processLimitAmounts()

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = err.(*billingpb.ResponseErrorMessage)
			return nil
		}
		break
	case pkg.OrderType_product:
		err = processor.processPaylinkProducts(ctx)

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = tokenErrorUnknown

			e, ok := err.(*billingpb.ResponseErrorMessage)

			if ok {
				rsp.Message = e
			}

			return nil
		}
		break
	case pkg.OrderType_key:
		err = processor.processPaylinkKeyProducts()

		if err != nil {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = tokenErrorUnknown

			e, ok := err.(*billingpb.ResponseErrorMessage)

			if ok {
				rsp.Message = e
			}

			return nil
		}
		break
	case pkg.OrderTypeVirtualCurrency:
		err := processor.processVirtualCurrency(ctx)
		if err != nil {
			zap.L().Error(
				pkg.MethodFinishedWithError,
				zap.Error(err),
			)

			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = err.(*billingpb.ResponseErrorMessage)
			return nil
		}
		break
	default:
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = tokenErrorSettingsTypeRequired
		return nil
	}

	project := processor.checked.project
	customer, err := s.findCustomer(ctx, req, project)

	if err != nil && err != customerNotFound {
		rsp.Status = billingpb.ResponseStatusBadData
		rsp.Message = err.(*billingpb.ResponseErrorMessage)
		return nil
	}

	if customer == nil {
		customer, err = s.createCustomer(ctx, req, project)
	} else {
		customer, err = s.updateCustomer(ctx, req, project, customer)
	}

	if err != nil {
		zap.S().Errorw(pkg.MethodFinishedWithError, "err", err)
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = e
			return nil
		}
		return err
	}

	token, err := s.createToken(req, customer)

	if err != nil {
		zap.S().Errorw(pkg.MethodFinishedWithError, "err", err)
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			rsp.Status = billingpb.ResponseStatusBadData
			rsp.Message = e
			return nil
		}
		return err
	}

	rsp.Status = billingpb.ResponseStatusOk
	rsp.Token = token

	return nil
}

func (s *Service) createToken(req *billingpb.TokenRequest, customer *billingpb.Customer) (string, error) {
	tokenRep := &tokenRepository{
		service: s,
		token: &Token{
			CustomerId: customer.Id,
			User:       req.User,
			Settings:   req.Settings,
		},
	}
	token := tokenRep.service.getTokenString(s.cfg.GetCustomerTokenLength())
	err := tokenRep.setToken(token)

	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) getTokenBy(token string) (*Token, error) {
	tokenRep := &tokenRepository{
		service: s,
		token:   &Token{},
	}
	err := tokenRep.getToken(token)

	if err != nil {
		return nil, err
	}

	return tokenRep.token, nil
}

func (s *Service) getCustomerById(ctx context.Context, id string) (*billingpb.Customer, error) {
	customer, err := s.customerRepository.GetById(ctx, id)

	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, orderErrorUnknown
		}

		return nil, customerNotFound
	}

	return customer, nil
}

func (s *Service) findCustomer(
	ctx context.Context,
	req *billingpb.TokenRequest,
	project *billingpb.Project,
) (*billingpb.Customer, error) {
	merchantId := ""

	if project != nil {
		merchantId = project.MerchantId
	}

	customer, err := s.customerRepository.Find(ctx, merchantId, req.User)

	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, orderErrorUnknown
		}

		return nil, customerNotFound
	}

	return customer, nil
}

func (s *Service) createCustomer(
	ctx context.Context,
	req *billingpb.TokenRequest,
	project *billingpb.Project,
) (*billingpb.Customer, error) {
	id := primitive.NewObjectID().Hex()

	customer := &billingpb.Customer{
		Id:        id,
		TechEmail: id + pkg.TechEmailDomain,
		Metadata:  req.User.Metadata,
		CreatedAt: ptypes.TimestampNow(),
		UpdatedAt: ptypes.TimestampNow(),
	}
	s.processCustomer(req, project, customer)

	if err := s.customerRepository.Insert(ctx, customer); err != nil {
		return nil, tokenErrorUnknown
	}

	return customer, nil
}

func (s *Service) updateCustomer(
	ctx context.Context,
	req *billingpb.TokenRequest,
	project *billingpb.Project,
	customer *billingpb.Customer,
) (*billingpb.Customer, error) {
	s.processCustomer(req, project, customer)

	if err := s.customerRepository.Update(ctx, customer); err != nil {
		return nil, tokenErrorUnknown
	}

	return customer, nil
}

func (s *Service) processCustomer(
	req *billingpb.TokenRequest,
	project *billingpb.Project,
	customer *billingpb.Customer,
) {
	user := req.User

	if user.Id != "" && user.Id != customer.ExternalId {
		customer.ExternalId = user.Id
		identity := &billingpb.CustomerIdentity{
			MerchantId: project.MerchantId,
			ProjectId:  project.Id,
			Type:       pkg.UserIdentityTypeExternal,
			Value:      user.Id,
			Verified:   true,
			CreatedAt:  ptypes.TimestampNow(),
		}

		customer.Identity = s.processCustomerIdentity(customer.Identity, identity)
	}

	if user.Email != nil && (customer.Email != user.Email.Value || customer.EmailVerified != user.Email.Verified) {
		customer.Email = user.Email.Value
		customer.EmailVerified = user.Email.Verified
		identity := &billingpb.CustomerIdentity{
			MerchantId: project.MerchantId,
			ProjectId:  project.Id,
			Type:       pkg.UserIdentityTypeEmail,
			Value:      user.Email.Value,
			Verified:   user.Email.Verified,
			CreatedAt:  ptypes.TimestampNow(),
		}

		customer.Identity = s.processCustomerIdentity(customer.Identity, identity)
	}

	if user.Phone != nil && (customer.Phone != user.Phone.Value || customer.PhoneVerified != user.Phone.Verified) {
		customer.Phone = user.Phone.Value
		customer.PhoneVerified = user.Phone.Verified
		identity := &billingpb.CustomerIdentity{
			MerchantId: project.MerchantId,
			ProjectId:  project.Id,
			Type:       pkg.UserIdentityTypePhone,
			Value:      user.Phone.Value,
			Verified:   user.Phone.Verified,
			CreatedAt:  ptypes.TimestampNow(),
		}

		customer.Identity = s.processCustomerIdentity(customer.Identity, identity)
	}

	if user.Name != nil && customer.Name != user.Name.Value {
		customer.Name = user.Name.Value
	}

	if user.Ip != nil && user.Ip.Value != "" {
		ip := net.IP(customer.Ip)
		customer.Ip = net.ParseIP(user.Ip.Value)

		if len(ip) > 0 && ip.String() != user.Ip.Value {
			history := &billingpb.CustomerIpHistory{
				Ip:        ip,
				CreatedAt: ptypes.TimestampNow(),
			}
			customer.IpHistory = append(customer.IpHistory, history)
		}
	}

	if user.Locale != nil && user.Locale.Value != "" && customer.Locale != user.Locale.Value {
		history := &billingpb.CustomerStringValueHistory{
			Value:     customer.Locale,
			CreatedAt: ptypes.TimestampNow(),
		}
		customer.Locale = user.Locale.Value

		if history.Value != "" {
			customer.LocaleHistory = append(customer.LocaleHistory, history)
		}
	}

	if user.Address != nil && customer.Address != user.Address {
		if customer.Address != nil {
			history := &billingpb.CustomerAddressHistory{
				Country:    customer.Address.Country,
				City:       customer.Address.City,
				PostalCode: customer.Address.PostalCode,
				State:      customer.Address.State,
				CreatedAt:  ptypes.TimestampNow(),
			}
			customer.AddressHistory = append(customer.AddressHistory, history)
		}

		customer.Address = user.Address
	}

	if user.UserAgent != "" && customer.UserAgent != user.UserAgent {
		customer.UserAgent = user.UserAgent
	}

	if user.AcceptLanguage != "" && customer.AcceptLanguage != user.AcceptLanguage {
		history := &billingpb.CustomerStringValueHistory{
			Value:     customer.AcceptLanguage,
			CreatedAt: ptypes.TimestampNow(),
		}
		customer.AcceptLanguage = user.AcceptLanguage

		if history.Value != "" {
			customer.AcceptLanguageHistory = append(customer.AcceptLanguageHistory, history)
		}
	}
}

func (s *Service) processCustomerIdentity(
	currentIdentities []*billingpb.CustomerIdentity,
	newIdentity *billingpb.CustomerIdentity,
) []*billingpb.CustomerIdentity {
	if len(currentIdentities) <= 0 {
		return append(currentIdentities, newIdentity)
	}

	isNewIdentity := true

	for k, v := range currentIdentities {
		needChange := v.Type == newIdentity.Type && v.ProjectId == newIdentity.ProjectId &&
			v.MerchantId == newIdentity.MerchantId && v.Value == newIdentity.Value && v.Verified != newIdentity.Verified

		if needChange == false {
			continue
		}

		currentIdentities[k] = newIdentity
		isNewIdentity = false
	}

	if isNewIdentity == true {
		currentIdentities = append(currentIdentities, newIdentity)
	}

	return currentIdentities
}

func (s *Service) transformOrderUser2TokenRequest(user *billingpb.OrderUser) *billingpb.TokenRequest {
	tokenReq := &billingpb.TokenRequest{User: &billingpb.TokenUser{}}

	if user.ExternalId != "" {
		tokenReq.User.Id = user.ExternalId
	}

	if user.Name != "" {
		tokenReq.User.Name = &billingpb.TokenUserValue{Value: user.Name}
	}

	if user.Email != "" {
		tokenReq.User.Email = &billingpb.TokenUserEmailValue{
			Value:    user.Email,
			Verified: user.EmailVerified,
		}
	}

	if user.Phone != "" {
		tokenReq.User.Phone = &billingpb.TokenUserPhoneValue{
			Value:    user.Phone,
			Verified: user.PhoneVerified,
		}
	}

	if user.Ip != "" {
		tokenReq.User.Ip = &billingpb.TokenUserIpValue{Value: user.Ip}
	}

	if user.Locale != "" {
		tokenReq.User.Locale = &billingpb.TokenUserLocaleValue{Value: user.Locale}
	}

	if user.Address != nil {
		tokenReq.User.Address = user.Address
	}

	if len(user.Metadata) > 0 {
		tokenReq.User.Metadata = user.Metadata
	}

	return tokenReq
}

func (r *tokenRepository) getToken(token string) error {
	data, err := r.service.redis.Get(r.getKey(token)).Bytes()

	if err != nil {
		r.service.logError("Get customer token from Redis failed", []interface{}{"error", err.Error()})
		return tokenErrorNotFound
	}

	err = json.Unmarshal(data, &r.token)

	if err != nil {
		r.service.logError("Unmarshal customer token failed", []interface{}{"error", err.Error()})
		return tokenErrorNotFound
	}

	return nil
}

func (r *tokenRepository) setToken(token string) error {
	b, err := json.Marshal(r.token)

	if err != nil {
		r.service.logError("Marshal customer token failed", []interface{}{"error", err.Error()})
		return tokenErrorUnknown
	}

	return r.service.redis.Set(r.getKey(token), b, r.service.cfg.GetCustomerTokenExpire()).Err()
}

func (r *tokenRepository) getKey(token string) string {
	return fmt.Sprintf(tokenStorageMask, token)
}

func (s *Service) getTokenString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)

	for i, cache, remain := n-1, tokenRandSource.Int63(), tokenLetterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = tokenRandSource.Int63(), tokenLetterIdxMax
		}

		if idx := int(uint64(cache) & tokenLetterIdxMask); idx < len(tokenLetterBytes) {
			sb.WriteByte(tokenLetterBytes[idx])
			i--
		}

		cache >>= tokenLetterIdxBits
		remain--
	}

	return sb.String()
}

func (s *Service) updateCustomerFromRequest(
	ctx context.Context,
	order *billingpb.Order,
	req *billingpb.TokenRequest,
	ip, acceptLanguage, userAgent string,
) (*billingpb.Customer, error) {
	customer, err := s.getCustomerById(ctx, order.User.Id)
	project := &billingpb.Project{Id: order.Project.Id, MerchantId: order.Project.MerchantId}

	if err != nil {
		return nil, err
	}

	req.User.Ip = &billingpb.TokenUserIpValue{Value: ip}
	req.User.AcceptLanguage = acceptLanguage
	req.User.UserAgent = userAgent

	req.User.Locale = &billingpb.TokenUserLocaleValue{}
	req.User.Locale.Value, _ = s.getCountryFromAcceptLanguage(acceptLanguage)

	return s.updateCustomer(ctx, req, project, customer)
}

func (s *Service) updateCustomerFromRequestLocale(
	ctx context.Context,
	order *billingpb.Order,
	ip, acceptLanguage, userAgent, locale string,
) {
	tokenReq := &billingpb.TokenRequest{
		User: &billingpb.TokenUser{
			Locale: &billingpb.TokenUserLocaleValue{Value: locale},
		},
	}

	_, err := s.updateCustomerFromRequest(ctx, order, tokenReq, ip, acceptLanguage, userAgent)

	if err != nil {
		zap.S().Errorf("Update customer data by request failed", "err", err.Error())
	}
}

func (s *Service) generateBrowserCookie(customer *BrowserCookieCustomer) (string, error) {
	b, err := json.Marshal(customer)

	if err != nil {
		zap.S().Errorf("Customer cookie generation failed", "err", err.Error())
		return "", err
	}

	hash := sha512.New()
	cookie, err := rsa.EncryptOAEP(hash, cryptoRand.Reader, s.cfg.CookiePublicKey, b, nil)

	if err != nil {
		zap.S().Errorf("Customer cookie generation failed", "err", err.Error())
		return "", err
	}

	return base64.StdEncoding.EncodeToString(cookie), nil
}

func (s *Service) decryptBrowserCookie(cookie string) (*BrowserCookieCustomer, error) {
	bCookie, err := base64.StdEncoding.DecodeString(cookie)

	if err != nil {
		zap.S().Errorf("Customer cookie base64 decode failed", "err", err.Error())
		return nil, err
	}

	hash := sha512.New()
	res, err := rsa.DecryptOAEP(hash, cryptoRand.Reader, s.cfg.CookiePrivateKey, bCookie, nil)

	if err != nil {
		zap.L().Error(
			"Customer cookie decrypt failed",
			zap.Error(err),
			zap.String("cookie", cookie),
		)
		return nil, err
	}

	customer := &BrowserCookieCustomer{}
	err = json.Unmarshal(res, &customer)

	if err != nil {
		zap.L().Error("Customer cookie decrypt failed", zap.Error(err))
		return nil, err
	}

	return customer, nil
}
