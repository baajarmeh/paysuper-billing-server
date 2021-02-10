package service

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/notifierpb"
	constant "github.com/paysuper/paysuper-proto/go/recurringpb"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
)

var (
	webhookTypeIncorrect = errors.NewBillingServerErrorMsg("wh000001", "type for webhook is invalid")
)

const (
	orderRequestType = "type"
)

func (s *Service) SendWebhookToMerchant(
	ctx context.Context,
	req *billingpb.OrderCreateRequest,
	res *billingpb.SendWebhookToMerchantResponse,
) error {
	res.Status = billingpb.ResponseStatusOk

	processor := &OrderCreateRequestProcessor{
		Service: s,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
		ctx:     ctx,
	}

	err := processor.processProject()

	if err != nil {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = err.(*billingpb.ResponseErrorMessage)
		return nil
	}

	switch req.Type {
	case pkg.OrderType_key:
		err = processor.processPaylinkKeyProducts()

		if err != nil {
			if pid := req.PrivateMetadata["PaylinkId"]; pid != "" {
				s.notifyPaylinkError(ctx, pid, err, req, nil)
			}

			if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
				res.Status = billingpb.ResponseStatusBadData
				res.Message = e
				return nil
			}

			return err
		}
		break
	case pkg.OrderType_product:
		err = processor.processPaylinkProducts(ctx)

		if err != nil {
			if pid := req.PrivateMetadata["PaylinkId"]; pid != "" {
				s.notifyPaylinkError(ctx, pid, err, req, nil)
			}

			if err == billingpb.ProductNoPriceInCurrencyError {
				res.Status = billingpb.ResponseStatusBadData
				res.Message = productNoPriceInCurrencyError
				return nil
			}

			if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
				res.Status = billingpb.ResponseStatusBadData
				res.Message = e
				return nil
			}

			return err
		}
		break
	case pkg.OrderType_simple:
		processor.processAmount()
	default:
		zap.L().Error(
			webhookTypeIncorrect.Message,
			zap.String(orderRequestType, req.Type),
		)
		res.Status = billingpb.ResponseStatusBadData
		res.Message = webhookTypeIncorrect
		res.Message.Details = req.Type
		return nil
	}

	if req.User != nil {
		err = processor.processUserData()

		if err != nil {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = err.(*billingpb.ResponseErrorMessage)
			return nil
		}
	}

	err = processor.processCurrency(req.Type)

	if err != nil {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = err.(*billingpb.ResponseErrorMessage)
		return nil
	}

	if processor.checked.currency == "" {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = orderErrorCurrencyIsRequired
		return nil
	}

	processor.processMetadata()
	processor.processPrivateMetadata()
	order, err := processor.prepareOrder()

	if err != nil {
		if e, ok := err.(*billingpb.ResponseErrorMessage); ok {
			res.Status = billingpb.ResponseStatusBadData
			res.Message = e
			return nil
		}
		return err
	}

	order.PrivateStatus = constant.OrderStatusPaymentSystemComplete

	var (
		topic   string
		payload proto.Message
		broker  rabbitmq.BrokerInterface
	)

	if (req.TestingCase == billingpb.TestCaseNonExistingUser || req.TestingCase == billingpb.TestCaseExistingUser) &&
		processor.checked.project.WebhookMode == pkg.ProjectWebhookPreApproval {
		if req.TestingCase == billingpb.TestCaseNonExistingUser {
			order.User.ExternalId = "paysuper_test_" + uuid.New().String()
		}

		topic = notifierpb.PayOneTopicNameValidateUser
		payload = s.getCheckUserRequestByOrder(order)
		payload.(*notifierpb.CheckUserRequest).TestingCase = req.TestingCase
		broker = s.validateUserBroker
	} else {
		topic = constant.PayOneTopicNotifyPaymentName
		payload = order
		broker = s.broker
	}

	err = broker.Publish(topic, payload, amqp.Table{"x-retry-count": int32(0)})

	if err != nil {
		zap.L().Error(
			brokerPublicationFailed,
			zap.Error(err),
			zap.String("topic", topic),
			zap.Any("payload", payload),
		)
	}

	res.OrderId = order.Uuid
	return nil
}

func (s *Service) NotifyWebhookTestResults(
	ctx context.Context,
	req *billingpb.NotifyWebhookTestResultsRequest,
	res *billingpb.EmptyResponseWithStatus,
) error {
	res.Status = billingpb.ResponseStatusOk

	s.mx.Lock()
	defer s.mx.Unlock()

	project, err := s.project.GetById(ctx, req.ProjectId)

	if err != nil {
		zap.L().Error(
			pkg.ErrorGrpcServiceCallFailed,
			zap.Error(err),
			zap.String(errorFieldService, "Project"),
			zap.String(errorFieldMethod, "GetById"),
		)
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = projectErrorUnknown
		return nil
	}

	if project.WebhookTesting == nil {
		project.WebhookTesting = &billingpb.WebHookTesting{}
	}

	switch req.Type {
	case pkg.OrderType_product:
		s.processTestingProducts(project, req)
		break
	case pkg.OrderType_key:
		s.processTestingKeys(project, req)
		break
	case pkg.OrderTypeVirtualCurrency:
		s.processTestingVirtualCurrency(project, req)
		break
	default:
		zap.L().Error(
			pkg.UnknownTypeError,
			zap.Error(err),
			zap.String(errorFieldService, "Project"),
			zap.String(errorFieldMethod, "GetById"),
		)
		res.Status = billingpb.ResponseStatusBadData
		res.Message = webhookTypeIncorrect
		res.Message.Details = req.Type
		return nil
	}

	err = s.project.Update(ctx, project)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = projectErrorUnknown
		return nil
	}

	return nil
}

func (s *Service) processTestingVirtualCurrency(project *billingpb.Project, req *billingpb.NotifyWebhookTestResultsRequest) {
	if project.WebhookTesting.VirtualCurrency == nil {
		project.WebhookTesting.VirtualCurrency = &billingpb.VirtualCurrencyTesting{}
	}

	switch req.TestCase {
	case pkg.TestCaseNonExistingUser:
		project.WebhookTesting.VirtualCurrency.NonExistingUser = req.IsPassed
		break
	case pkg.TestCaseExistingUser:
		project.WebhookTesting.VirtualCurrency.ExistingUser = req.IsPassed
		break
	case pkg.TestCaseCorrectPayment:
		project.WebhookTesting.VirtualCurrency.CorrectPayment = req.IsPassed
		break
	case pkg.TestCaseIncorrectPayment:
		project.WebhookTesting.VirtualCurrency.IncorrectPayment = req.IsPassed
		break
	}
}

func (s *Service) processTestingKeys(project *billingpb.Project, req *billingpb.NotifyWebhookTestResultsRequest) {
	if project.WebhookTesting.Keys == nil {
		project.WebhookTesting.Keys = &billingpb.KeysTesting{}
	}
	project.WebhookTesting.Keys.IsPassed = req.IsPassed
}

func (s *Service) processTestingProducts(project *billingpb.Project, req *billingpb.NotifyWebhookTestResultsRequest) {
	if project.WebhookTesting.Products == nil {
		project.WebhookTesting.Products = &billingpb.ProductsTesting{}
	}
	switch req.TestCase {
	case pkg.TestCaseNonExistingUser:
		project.WebhookTesting.Products.NonExistingUser = req.IsPassed
		break
	case pkg.TestCaseExistingUser:
		project.WebhookTesting.Products.ExistingUser = req.IsPassed
		break
	case pkg.TestCaseCorrectPayment:
		project.WebhookTesting.Products.CorrectPayment = req.IsPassed
		break
	case pkg.TestCaseIncorrectPayment:
		project.WebhookTesting.Products.IncorrectPayment = req.IsPassed
		break
	}
}

func (s *Service) getCheckUserRequestByOrder(order *billingpb.Order) *notifierpb.CheckUserRequest {
	req := &notifierpb.CheckUserRequest{
		Url:           order.Project.UrlProcessPayment,
		SecretKey:     order.Project.GetSecretKey(),
		IsLiveProject: order.Project.Status == billingpb.ProjectStatusInProduction,
		User: &notifierpb.User{
			Id:                   order.User.Id,
			Object:               order.User.Object,
			ExternalId:           order.User.ExternalId,
			Name:                 order.User.Name,
			Email:                order.User.Email,
			EmailVerified:        order.User.EmailVerified,
			Phone:                order.User.Phone,
			PhoneVerified:        order.User.PhoneVerified,
			Ip:                   order.User.Ip,
			Locale:               order.User.Locale,
			Metadata:             order.User.Metadata,
			NotifyNewRegion:      order.User.NotifyNewRegion,
			NotifyNewRegionEmail: order.User.NotifyNewRegionEmail,
		},
		OrderUuid:  order.Uuid,
		MerchantId: order.Project.MerchantId,
	}

	if order.User.Address != nil {
		req.User.Address = &notifierpb.BillingAddress{
			Country:    order.User.Address.Country,
			City:       order.User.Address.City,
			PostalCode: order.User.Address.PostalCode,
			State:      order.User.Address.State,
		}
	}

	return req
}

func (s *Service) webhookCheckUser(order *billingpb.Order) error {
	req := s.getCheckUserRequestByOrder(order)
	rsp, err := s.notifier.CheckUser(context.TODO(), req)

	if err != nil {
		zap.L().Error(
			pkg.ErrorGrpcServiceCallFailed,
			zap.Error(err),
			zap.String(errorFieldService, notifierpb.ServiceName),
			zap.String(errorFieldMethod, "CheckUser"),
		)
		return orderErrorMerchantUserAccountNotChecked
	}

	if rsp.Status != billingpb.ResponseStatusOk {
		zap.L().Error(
			pkg.ErrorUserCheckFailed,
			zap.Int32(errorFieldStatus, rsp.Status),
			zap.String(errorFieldMessage, rsp.Message),
		)
		return orderErrorMerchantUserAccountNotChecked
	}

	return nil
}
