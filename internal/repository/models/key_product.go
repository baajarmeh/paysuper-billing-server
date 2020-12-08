package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	tools "github.com/paysuper/paysuper-tools/number"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type keyProductMapper struct{}

func NewKeyProductMapper() Mapper {
	return &keyProductMapper{}
}

type MgoKeyProduct struct {
	Id                 primitive.ObjectID              `bson:"_id" json:"id" faker:"objectId"`
	Object             string                          `bson:"object" json:"object"`
	Sku                string                          `bson:"sku" json:"sku"`
	Name               []*billingpb.I18NTextSearchable `bson:"name" json:"name"`
	DefaultCurrency    string                          `bson:"default_currency" json:"default_currency"`
	Enabled            bool                            `bson:"enabled" json:"enabled"`
	Platforms          []*MgoPlatformPrice             `bson:"platforms" json:"platforms"`
	Description        map[string]string               `bson:"description" json:"description"`
	LongDescription    map[string]string               `bson:"long_description,omitempty" json:"long_description"`
	CreatedAt          time.Time                       `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time                       `bson:"updated_at" json:"updated_at"`
	PublishedAt        *time.Time                      `bson:"published_at" json:"published_at"`
	Cover              *billingpb.ImageCollection      `bson:"cover" json:"cover"`
	Url                string                          `bson:"url,omitempty" json:"url"`
	Metadata           map[string]string               `bson:"metadata,omitempty" json:"metadata"`
	Deleted            bool                            `bson:"deleted" json:"deleted"`
	MerchantId         primitive.ObjectID              `bson:"merchant_id" json:"-" faker:"objectId"`
	ProjectId          primitive.ObjectID              `bson:"project_id" json:"project_id" faker:"objectId"`
	Pricing            string                          `bson:"pricing" json:"pricing"`
	MinimalLimitNotify int32                           `bson:"minimal_limit_notify" json:"minimal_limit_notify"`
}

type MgoPlatformPrice struct {
	Prices        []*billingpb.ProductPrice `bson:"prices" json:"prices"`
	Id            string                    `bson:"id" json:"id"`
	Name          string                    `bson:"name" json:"name"`
	EulaUrl       string                    `bson:"eula_url" json:"eula_url"`
	ActivationUrl string                    `bson:"activation_url" json:"activation_url"`
	Count         int32                     `bson:"count" json:"count"`
}

func (m *keyProductMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.KeyProduct)

	out := &MgoKeyProduct{
		Object:             in.Object,
		Sku:                in.Sku,
		DefaultCurrency:    in.DefaultCurrency,
		Enabled:            in.Enabled,
		Description:        in.Description,
		LongDescription:    in.LongDescription,
		Cover:              in.Cover,
		Url:                in.Url,
		Metadata:           in.Metadata,
		Deleted:            in.Deleted,
		Pricing:            in.Pricing,
		MinimalLimitNotify: in.MinimalLimitNotify,
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

	if in.PublishedAt != nil {
		t, err := ptypes.Timestamp(in.PublishedAt)

		if err != nil {
			return nil, err
		}

		out.PublishedAt = &t
	}

	merchantOid, err := primitive.ObjectIDFromHex(in.MerchantId)

	if err != nil {
		return nil, err
	}

	out.MerchantId = merchantOid

	projectOid, err := primitive.ObjectIDFromHex(in.ProjectId)

	if err != nil {
		return nil, err
	}

	out.ProjectId = projectOid

	out.Name = make([]*billingpb.I18NTextSearchable, len(in.Name))
	index := 0
	for k, v := range in.Name {
		out.Name[index] = &billingpb.I18NTextSearchable{Lang: k, Value: v}
		index++
	}

	out.Platforms = make([]*MgoPlatformPrice, len(in.Platforms))
	for i, pl := range in.Platforms {
		var prices []*billingpb.ProductPrice
		prices = make([]*billingpb.ProductPrice, len(pl.Prices))

		for j, price := range pl.Prices {
			prices[j] = &billingpb.ProductPrice{
				Currency:          price.Currency,
				Region:            price.Region,
				Amount:            tools.FormatAmount(price.Amount),
				IsVirtualCurrency: price.IsVirtualCurrency,
			}
		}
		out.Platforms[i] = &MgoPlatformPrice{
			Prices:        prices,
			Id:            pl.Id,
			Name:          pl.Name,
			EulaUrl:       pl.EulaUrl,
			ActivationUrl: pl.ActivationUrl,
			Count:         pl.Count,
		}
	}

	return out, nil
}

func (m *keyProductMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoKeyProduct)

	out := &billingpb.KeyProduct{
		Id:                 in.Id.Hex(),
		Object:             in.Object,
		Sku:                in.Sku,
		DefaultCurrency:    in.DefaultCurrency,
		Enabled:            in.Enabled,
		Description:        in.Description,
		LongDescription:    in.LongDescription,
		Cover:              in.Cover,
		Url:                in.Url,
		Metadata:           in.Metadata,
		Deleted:            in.Deleted,
		MerchantId:         in.MerchantId.Hex(),
		ProjectId:          in.ProjectId.Hex(),
		Pricing:            in.Pricing,
		MinimalLimitNotify: in.MinimalLimitNotify,
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)

	if err != nil {
		return nil, err
	}

	out.UpdatedAt, err = ptypes.TimestampProto(in.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if in.PublishedAt != nil {
		out.PublishedAt, err = ptypes.TimestampProto(*in.PublishedAt)
		if err != nil {
			return nil, err
		}
	}

	platforms := make([]*billingpb.PlatformPrice, len(in.Platforms))
	for i, pl := range in.Platforms {
		platforms[i] = &billingpb.PlatformPrice{
			Id:            pl.Id,
			Prices:        pl.Prices,
			EulaUrl:       pl.EulaUrl,
			Name:          pl.Name,
			ActivationUrl: pl.ActivationUrl,
			Count:         pl.Count,
		}
	}

	out.Platforms = platforms

	out.Name = map[string]string{}
	for _, i := range in.Name {
		out.Name[i.Lang] = i.Value
	}

	return out, nil
}
