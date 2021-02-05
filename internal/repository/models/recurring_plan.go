package models

import (
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type recurringPlanMapper struct{}

func NewRecurringPlanMapper() Mapper {
	return &recurringPlanMapper{}
}

type MgoRecurringPlan struct {
	Id          primitive.ObjectID              `bson:"_id" faker:"objectId"`
	MerchantId  primitive.ObjectID              `bson:"merchant_id" faker:"objectId"`
	ProjectId   primitive.ObjectID              `bson:"project_id" faker:"objectId"`
	Name        []*billingpb.I18NTextSearchable `bson:"name"`
	Description []*billingpb.I18NTextSearchable `bson:"description"`
	Charge      *MgoRecurringPlanCharge         `bson:"charge"`
	ExternalId  string                          `bson:"external_id"`
	GroupId     string                          `bson:"group_id"`
	Expiration  *MgoRecurringPlanPeriod         `bson:"expiration"`
	Trial       *MgoRecurringPlanPeriod         `bson:"trial"`
	GracePeriod *MgoRecurringPlanPeriod         `bson:"grace_period"`
	Tags        []string                        `bson:"tags"`
	Status      string                          `bson:"status"`
	CreatedAt   time.Time                       `bson:"created_at"`
	UpdatedAt   time.Time                       `bson:"updated_at"`
	DeletedAt   *time.Time                      `bson:"deleted_at"`
}

type MgoRecurringPlanCharge struct {
	Period   *MgoRecurringPlanPeriod `bson:"period"`
	Amount   float64                 `bson:"amount"`
	Currency string                  `bson:"currency"`
}

type MgoRecurringPlanPeriod struct {
	Value int32  `bson:"value"`
	Type  string `bson:"type"`
}

func (m *recurringPlanMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.RecurringPlan)

	out := &MgoRecurringPlan{
		ExternalId: in.ExternalId,
		GroupId:    in.GroupId,
		Tags:       in.Tags,
		Status:     in.Status,
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

	oid, err := primitive.ObjectIDFromHex(in.MerchantId)
	if err != nil {
		return nil, err
	}
	out.MerchantId = oid

	oid, err = primitive.ObjectIDFromHex(in.ProjectId)
	if err != nil {
		return nil, err
	}
	out.ProjectId = oid

	out.Name = []*billingpb.I18NTextSearchable{}
	for k, v := range in.Name {
		out.Name = append(out.Name, &billingpb.I18NTextSearchable{Lang: k, Value: v})
	}

	out.Description = []*billingpb.I18NTextSearchable{}
	for k, v := range in.Description {
		out.Description = append(out.Description, &billingpb.I18NTextSearchable{Lang: k, Value: v})
	}

	if in.Charge == nil || in.Charge.Period == nil {
		return nil, fmt.Errorf("invalid charge of recurring plan")
	} else {
		out.Charge = &MgoRecurringPlanCharge{
			Period: &MgoRecurringPlanPeriod{
				Value: in.Charge.Period.Value,
				Type:  in.Charge.Period.Type,
			},
			Amount:   in.Charge.Amount,
			Currency: in.Charge.Currency,
		}
	}

	if in.Expiration != nil {
		out.Expiration = &MgoRecurringPlanPeriod{
			Value: in.Expiration.Value,
			Type:  in.Expiration.Type,
		}
	}

	if in.Trial != nil {
		out.Trial = &MgoRecurringPlanPeriod{
			Value: in.Trial.Value,
			Type:  in.Trial.Type,
		}
	}

	if in.GracePeriod != nil {
		out.GracePeriod = &MgoRecurringPlanPeriod{
			Value: in.GracePeriod.Value,
			Type:  in.GracePeriod.Type,
		}
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

	if in.DeletedAt != nil {
		t, err := ptypes.Timestamp(in.DeletedAt)

		if err != nil {
			return nil, err
		}

		out.DeletedAt = &t
	}

	return out, nil
}

func (m *recurringPlanMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoRecurringPlan)

	out := &billingpb.RecurringPlan{
		Id:         in.Id.Hex(),
		MerchantId: in.MerchantId.Hex(),
		ProjectId:  in.ProjectId.Hex(),
		ExternalId: in.ExternalId,
		GroupId:    in.GroupId,
		Tags:       in.Tags,
		Status:     in.Status,
	}

	out.Name = map[string]string{}
	for _, i := range in.Name {
		out.Name[i.Lang] = i.Value
	}

	out.Description = map[string]string{}
	for _, i := range in.Description {
		out.Description[i.Lang] = i.Value
	}

	if in.Charge != nil && in.Charge.Period != nil {
		out.Charge = &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: in.Charge.Period.Value,
				Type:  in.Charge.Period.Type,
			},
			Amount:   in.Charge.Amount,
			Currency: in.Charge.Currency,
		}
	}

	if in.Expiration != nil {
		out.Expiration = &billingpb.RecurringPlanPeriod{
			Value: in.Expiration.Value,
			Type:  in.Expiration.Type,
		}
	}

	if in.Trial != nil {
		out.Trial = &billingpb.RecurringPlanPeriod{
			Value: in.Trial.Value,
			Type:  in.Trial.Type,
		}
	}

	if in.GracePeriod != nil {
		out.GracePeriod = &billingpb.RecurringPlanPeriod{
			Value: in.GracePeriod.Value,
			Type:  in.GracePeriod.Type,
		}
	}

	if in.DeletedAt != nil {
		out.DeletedAt, err = ptypes.TimestampProto(*in.DeletedAt)
		if err != nil {
			return nil, err
		}
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)
	if err != nil {
		return nil, err
	}

	out.UpdatedAt, err = ptypes.TimestampProto(in.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return out, nil
}
