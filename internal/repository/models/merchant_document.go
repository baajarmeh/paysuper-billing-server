package models

import (
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type merchantDocumentMapper struct {
}

type MgoMerchantDocument struct {
	Id           primitive.ObjectID `bson:"_id" faker:"objectId"`
	MerchantId   primitive.ObjectID `bson:"merchant_id" faker:"objectId"`
	UserId       primitive.ObjectID `bson:"user_id" faker:"objectId"`
	OriginalName string             `bson:"original_name"`
	FilePath     string             `bson:"file_path"`
	Description  string             `bson:"description"`
	CreatedAt    time.Time          `bson:"created_at"`
}

func (h *merchantDocumentMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	m := obj.(*billingpb.MerchantDocument)
	st := &MgoMerchantDocument{
		OriginalName: m.OriginalName,
		FilePath:     m.FilePath,
		Description:  m.Description,
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

	merchantOid, err := primitive.ObjectIDFromHex(m.MerchantId)

	if err != nil {
		return nil, errors.New(billingpb.ErrorInvalidObjectId)
	}

	st.MerchantId = merchantOid

	userOid, err := primitive.ObjectIDFromHex(m.UserId)

	if err != nil {
		return nil, errors.New(billingpb.ErrorInvalidObjectId)
	}

	st.UserId = userOid

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

func (h *merchantDocumentMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	decoded := obj.(*MgoMerchantDocument)
	m := &billingpb.MerchantDocument{}

	m.Id = decoded.Id.Hex()
	m.MerchantId = decoded.MerchantId.Hex()
	m.UserId = decoded.UserId.Hex()
	m.OriginalName = decoded.OriginalName
	m.FilePath = decoded.FilePath
	m.Description = decoded.Description

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func NewMerchantDocumentMapper() Mapper {
	return &merchantDocumentMapper{}
}
