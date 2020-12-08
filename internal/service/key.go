package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/errors"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/paysuper/paysuper-proto/go/postmarkpb"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

const (
	minimalKeysNotifyKey = "key:minimal:notify:%s:%s"
	emptyKeysNotifyKey   = "key:empty:notify:%s:%s"
)

var (
	minimalKeyNotificationMessage = "Your keys are running out! There are only %d keys left for %s on %s."
	emptyKeyNotificationMessage   = "You’re all out! There are no more keys available for %s on %s."
)

func (s *Service) UploadKeysFile(
	ctx context.Context,
	req *billingpb.PlatformKeysFileRequest,
	res *billingpb.PlatformKeysFileResponse,
) error {
	scanner := bufio.NewScanner(bytes.NewReader(req.File))
	count, err := s.keyRepository.CountKeysByProductPlatform(ctx, req.KeyProductId, req.PlatformId)

	if err != nil {
		zap.S().Errorf(errors.KeyErrorNotFound.Message, "err", err.Error(), "keyProductId", req.KeyProductId, "platformId", req.PlatformId)
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = errors.KeyErrorNotFound
		return nil
	}

	res.TotalCount = int32(count)

	// Process key by line
	for scanner.Scan() {
		key := &billingpb.Key{
			Id:           primitive.NewObjectID().Hex(),
			Code:         scanner.Text(),
			KeyProductId: req.KeyProductId,
			PlatformId:   req.PlatformId,
		}

		if err := s.keyRepository.Insert(ctx, key); err != nil {
			zap.S().Errorf(errors.KeyErrorFailedToInsert.Message, "err", err, "key", key)
			continue
		}

		res.TotalCount++
		res.KeysProcessed++
	}

	// tell about errors
	if err = scanner.Err(); err != nil {
		zap.S().Errorf(errors.KeyErrorFileProcess.Message, "err", err.Error())
		res.Message = errors.KeyErrorFileProcess
		res.Status = billingpb.ResponseStatusBadData
		return nil
	}

	emptyStorageKey := fmt.Sprintf(emptyKeysNotifyKey, req.KeyProductId, req.PlatformId)
	minimalStorageKey := fmt.Sprintf(minimalKeysNotifyKey, req.KeyProductId, req.PlatformId)

	err = s.redis.Del(emptyStorageKey, minimalStorageKey).Err()

	if err != nil {
		zap.L().Error(
			"unable to delete key product notification keys",
			zap.Error(err),
			zap.String("empty_storage_key", emptyStorageKey),
			zap.String("minimal_storage_key", minimalStorageKey),
		)
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetAvailableKeysCount(
	ctx context.Context,
	req *billingpb.GetPlatformKeyCountRequest,
	res *billingpb.GetPlatformKeyCountResponse,
) error {
	keyProduct, err := s.keyProductRepository.GetById(ctx, req.KeyProductId)

	if err != nil {
		zap.S().Errorf(keyProductNotFound.Message, "err", err.Error(), "keyProductId", req.KeyProductId, "platformId", req.PlatformId)
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = keyProductNotFound
		return nil
	}

	if keyProduct.MerchantId != req.MerchantId {
		zap.S().Error(keyProductMerchantMismatch.Message, "keyProductId", req.KeyProductId)
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = keyProductMerchantMismatch
		return nil
	}

	count, err := s.keyRepository.CountKeysByProductPlatform(ctx, req.KeyProductId, req.PlatformId)

	if err != nil {
		zap.S().Errorf(errors.KeyErrorNotFound.Message, "err", err.Error(), "keyProductId", req.KeyProductId, "platformId", req.PlatformId)
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = errors.KeyErrorNotFound
		return nil
	}

	res.Count = int32(count)
	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) GetKeyByID(
	ctx context.Context,
	req *billingpb.KeyForOrderRequest,
	res *billingpb.GetKeyForOrderRequestResponse,
) error {
	key, err := s.keyRepository.GetById(ctx, req.KeyId)

	if err != nil {
		zap.S().Errorf(errors.KeyErrorNotFound.Message, "err", err.Error(), "keyId", req.KeyId)
		res.Status = billingpb.ResponseStatusNotFound
		res.Message = errors.KeyErrorNotFound
		return nil
	}

	res.Key = key

	return nil
}

func (s *Service) ReserveKeyForOrder(
	ctx context.Context,
	req *billingpb.PlatformKeyReserveRequest,
	res *billingpb.PlatformKeyReserveResponse,
) error {
	zap.S().Infow("[ReserveKeyForOrder] called", "order_id", req.OrderId, "platform_id", req.PlatformId, "KeyProductId", req.KeyProductId)
	key, err := s.keyRepository.ReserveKey(ctx, req.KeyProductId, req.PlatformId, req.OrderId, req.Ttl)

	if err != nil {
		res.Status = billingpb.ResponseStatusBadData
		res.Message = errors.KeyErrorReserve
		return nil
	}

	zap.S().Infow("[ReserveKeyForOrder] reserved key", "req.order_id", req.OrderId, "key.order_id", key.OrderId, "key.id", key.Id, "key.RedeemedAt", key.RedeemedAt, "key.KeyProductId", key.KeyProductId)

	res.KeyId = key.Id
	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) FinishRedeemKeyForOrder(
	ctx context.Context,
	req *billingpb.KeyForOrderRequest,
	res *billingpb.GetKeyForOrderRequestResponse,
) error {
	key, err := s.keyRepository.FinishRedeemById(ctx, req.KeyId)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errors.KeyErrorFinish
		return nil
	}

	res.Key = key
	res.Status = billingpb.ResponseStatusOk

	go func() {
		_ = s.checkAndNotifyProductKeys(key)
	}()

	return nil
}

func (s *Service) checkAndNotifyProductKeys(key *billingpb.Key) error {
	ctx := context.Background()

	keyProduct, err := s.keyProductRepository.GetById(ctx, key.KeyProductId)

	if err != nil {
		return err
	}

	count, err := s.keyRepository.CountKeysByProductPlatform(ctx, key.KeyProductId, key.PlatformId)

	if err != nil {
		return err
	}

	if count > int64(keyProduct.MinimalLimitNotify) {
		return nil
	}

	var (
		storageKey        string
		templateName      string
		centrifugoMessage string
	)

	if count <= 0 {
		storageKey = fmt.Sprintf(emptyKeysNotifyKey, key.KeyProductId, key.PlatformId)
		templateName = s.cfg.EmptyKeyProductNotify
		centrifugoMessage = fmt.Sprintf(
			emptyKeyNotificationMessage,
			keyProduct.Name["en"],
			availablePlatforms[key.PlatformId].Name,
		)
	} else {
		storageKey = fmt.Sprintf(minimalKeysNotifyKey, key.KeyProductId, key.PlatformId)
		templateName = s.cfg.MinimalKeyProductNotify
		centrifugoMessage = fmt.Sprintf(
			minimalKeyNotificationMessage,
			keyProduct.MinimalLimitNotify,
			keyProduct.Name["en"],
			availablePlatforms[key.PlatformId].Name,
		)
	}

	redisRes := s.redis.Exists(storageKey)

	if redisRes.Err() != nil {
		zap.L().Error(
			"[checkAndNotifyProductKeys] unable to get value from the Redis",
			zap.Error(redisRes.Err()),
			zap.String("storage_key", storageKey),
		)

		return err
	}

	if redisRes.Val() != 0 {
		return nil
	}

	project, err := s.project.GetById(ctx, keyProduct.ProjectId)

	if err != nil {
		zap.L().Error(
			"[checkAndNotifyProductKeys] unable to get project",
			zap.Error(err),
			zap.String("project_id", keyProduct.ProjectId),
		)

		return err
	}

	payload := &postmarkpb.Payload{
		TemplateAlias: templateName,
		TemplateModel: map[string]string{
			"project_name":   project.Name["en"],
			"product_name":   keyProduct.Name["en"],
			"platform_name":  availablePlatforms[key.PlatformId].Name,
			"minimal_limit":  fmt.Sprintf("%d", keyProduct.MinimalLimitNotify),
			"key_upload_url": fmt.Sprintf(pkg.UploadProductKeysUrl, s.cfg.DashboardUrl, project.Id, keyProduct.Id),
		},
		To: s.cfg.EmailOnboardingAdminRecipient,
	}

	err = s.postmarkBroker.Publish(postmarkpb.PostmarkSenderTopicName, payload, amqp.Table{})
	if err != nil {
		zap.L().Error(
			"Can't send email",
			zap.Error(err),
			zap.Any("payload", payload),
		)

		return err
	}

	msg := map[string]interface{}{"message": centrifugoMessage}
	err = s.centrifugoDashboard.Publish(ctx, fmt.Sprintf(s.cfg.CentrifugoMerchantChannel, project.MerchantId), msg)

	if err != nil {
		zap.L().Error(
			"Can't send centrifugo message",
			zap.Error(err),
			zap.Any("msg", msg),
		)

		return err
	}

	if err = s.redis.Set(storageKey, 1, 0).Err(); err != nil {
		zap.L().Error(
			"[checkAndNotifyProductKeys] unable to set key product notify to the Redis",
			zap.Error(err),
			zap.String("storage_key", storageKey),
		)

		return err
	}

	return err
}

func (s *Service) CancelRedeemKeyForOrder(
	ctx context.Context,
	req *billingpb.KeyForOrderRequest,
	res *billingpb.EmptyResponseWithStatus,
) error {
	_, err := s.keyRepository.CancelById(ctx, req.KeyId)

	if err != nil {
		res.Status = billingpb.ResponseStatusSystemError
		res.Message = errors.KeyErrorCanceled
		return nil
	}

	res.Status = billingpb.ResponseStatusOk

	return nil
}

func (s *Service) KeyDaemonProcess(ctx context.Context) (int, error) {
	counter := 0
	keys, err := s.keyRepository.FindUnfinished(ctx)

	if err != nil {
		return counter, err
	}

	for _, key := range keys {
		_, err = s.keyRepository.CancelById(ctx, key.Id)

		if err != nil {
			continue
		}

		counter++
	}

	return counter, nil
}
