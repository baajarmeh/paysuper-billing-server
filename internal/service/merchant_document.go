package service

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/postmarkpb"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

var (
	errorMerchantDocumentNotFound        = errors.NewBillingServerErrorMsg("md000001", "unable to get merchant document list")
	errorMerchantDocumentAccessDenied    = errors.NewBillingServerErrorMsg("md000002", "access denied")
	errorMerchantDocumentUnableInsert    = errors.NewBillingServerErrorMsg("md000003", "unable to add merchant document")
	errorMerchantDocumentIncorrectStatus = errors.NewBillingServerErrorMsg("md000004", "incorrect merchant status for document upload")
	errorMerchantNotFound                = errors.NewBillingServerErrorMsg("md000005", "merchant not found")
)

func (s *Service) AddMerchantDocument(
	ctx context.Context,
	req *billingpb.MerchantDocument,
	res *billingpb.AddMerchantDocumentResponse,
) error {
	merchant, err := s.merchantRepository.GetById(ctx, req.MerchantId)

	if err != nil {
		zap.L().Error("incorrect merchant status for document upload", zap.Error(err), zap.String("merchant_id", req.MerchantId))
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorMerchantNotFound
		return nil
	}

	if merchant.Status != billingpb.MerchantStatusKycStarted {
		if !merchant.CanChangeStatusTo(billingpb.MerchantStatusKycStarted) {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = errorMerchantDocumentIncorrectStatus
			return nil
		}

		merchant.Status = billingpb.MerchantStatusKycStarted
		merchant.StatusLastUpdatedAt = ptypes.TimestampNow()
		err = s.merchantRepository.Update(ctx, merchant)

		if err != nil {
			res.Status = billingpb.ResponseStatusSystemError
			res.Message = errorMerchantDocumentUnableInsert

			return nil
		}
	}

	req.Id = primitive.NewObjectID().Hex()
	err = s.merchantDocumentRepository.Insert(ctx, req)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorMerchantDocumentUnableInsert
		return nil
	}

	models := map[string]string{
		"merchant_name":          merchant.Company.Name,
		"merchant_dashboard_url": fmt.Sprintf(pkg.MerchantKycListUrl, s.cfg.DashboardUrl),
		"admin_dashboard_url":    fmt.Sprintf(pkg.AdminKycListUrl, s.cfg.DashboardUrl, req.MerchantId),
	}

	merchantPayload := &postmarkpb.Payload{
		TemplateAlias: s.cfg.EmailTemplates.MerchantDocumentUploaded,
		TemplateModel: models,
		To:            merchant.User.Email,
	}

	err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, merchantPayload, amqp.Table{})
	if err != nil {
		zap.L().Error("can't send email", zap.Error(err), zap.Any("payload", merchantPayload))
	}

	adminPayload := &postmarkpb.Payload{
		TemplateAlias: s.cfg.EmailTemplates.AdminDocumentUploaded,
		TemplateModel: models,
		To:            s.cfg.EmailOnboardingAdminRecipient,
	}

	err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, adminPayload, amqp.Table{})
	if err != nil {
		zap.L().Error("can't send email", zap.Error(err), zap.Any("payload", adminPayload))
	}

	res.Item = req
	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetMerchantDocument(
	ctx context.Context,
	req *billingpb.GetMerchantDocumentRequest,
	res *billingpb.GetMerchantDocumentResponse,
) error {
	item, err := s.merchantDocumentRepository.GetById(ctx, req.Id)

	if err != nil {
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = errorMerchantDocumentNotFound
		return nil
	}

	if item.MerchantId != req.MerchantId {
		res.Status = billingpb.ResponseStatusForbidden
		res.Message = errorMerchantDocumentAccessDenied
		return nil
	}

	res.Item = item
	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetMerchantDocuments(
	ctx context.Context,
	req *billingpb.GetMerchantDocumentsRequest,
	res *billingpb.GetMerchantDocumentsResponse,
) error {
	var err error

	res.List, err = s.merchantDocumentRepository.GetByMerchantId(ctx, req.MerchantId, req.Offset, req.Limit)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorMerchantDocumentNotFound
		return nil
	}

	res.Count, err = s.merchantDocumentRepository.CountByMerchantId(ctx, req.MerchantId)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errorMerchantDocumentNotFound
		return nil
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}
