package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type recurringSubscriptionMapper struct{}

func NewRecurringSubscriptionMapper() Mapper {
	return &recurringSubscriptionMapper{}
}

type MgoRecurringSubscription struct {
	Id                    primitive.ObjectID    `bson:"_id"`
	Plan                  *MgoRecurringPlan     `bson:"plan"`
	Customer              *MgoRecurringCustomer `bson:"customer"`
	Project               *MgoRecurringProject  `bson:"project"`
	Status                string                `bson:"status"`
	TotalAmount           float64               `bson:"total_amount"`
	ItemType              string                `bson:"item_type"`
	ItemList              []string              `bson:"item_list"`
	CardPayPlanId         string                `bson:"cardpay_plan_id"`
	CardPaySubscriptionId string                `bson:"cardpay_subscription_id"`
	ExpireAt              *time.Time            `bson:"expire_at"`
	LastPaymentAt         *time.Time            `bson:"last_payment_at"`
	CreatedAt             time.Time             `bson:"created_at"`
	UpdatedAt             time.Time             `bson:"updated_at"`
}

type MgoRecurringCustomer struct {
	Id         primitive.ObjectID `bson:"id"`
	Uuid       string             `bson:"uuid"`
	ExternalId string             `bson:"external_id"`
	Email      string             `bson:"email"`
	Phone      string             `bson:"phone"`
}

type MgoRecurringProject struct {
	Id   primitive.ObjectID `bson:"id"`
	Name []*MgoMultiLang    `bson:"name"`
}

func (m *recurringSubscriptionMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.RecurringSubscription)

	out := &MgoRecurringSubscription{
		Status:                in.Status,
		TotalAmount:           in.TotalAmount,
		ItemType:              in.ItemType,
		ItemList:              in.ItemList,
		CardPayPlanId:         in.CardpayPlanId,
		CardPaySubscriptionId: in.CardpaySubscriptionId,
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

	plan, err := NewRecurringPlanMapper().MapObjectToMgo(in.Plan)

	if err != nil {
		return nil, err
	}

	out.Plan = plan.(*MgoRecurringPlan)

	customerId, err := primitive.ObjectIDFromHex(in.Customer.Id)

	if err != nil {
		return nil, err
	}

	out.Customer = &MgoRecurringCustomer{
		Id:         customerId,
		Uuid:       in.Customer.Uuid,
		ExternalId: in.Customer.ExternalId,
		Email:      in.Customer.Email,
		Phone:      in.Customer.Phone,
	}

	projectId, err := primitive.ObjectIDFromHex(in.Project.Id)
	if err != nil {
		return nil, err
	}

	out.Project = &MgoRecurringProject{
		Id: projectId,
	}

	for k, v := range in.Project.Name {
		out.Project.Name = append(out.Project.Name, &MgoMultiLang{Lang: k, Value: v})
	}

	if in.LastPaymentAt != nil {
		t, err := ptypes.Timestamp(in.LastPaymentAt)

		if err != nil {
			return nil, err
		}

		out.LastPaymentAt = &t
	}

	if in.ExpireAt != nil {
		t, err := ptypes.Timestamp(in.ExpireAt)

		if err != nil {
			return nil, err
		}

		out.ExpireAt = &t
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

	return out, nil
}

func (m *recurringSubscriptionMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoRecurringSubscription)

	out := &billingpb.RecurringSubscription{
		Id:                    in.Id.Hex(),
		Status:                in.Status,
		TotalAmount:           in.TotalAmount,
		ItemType:              in.ItemType,
		ItemList:              in.ItemList,
		CardpayPlanId:         in.CardPayPlanId,
		CardpaySubscriptionId: in.CardPaySubscriptionId,
		Customer: &billingpb.RecurringSubscriptionCustomer{
			Id:         in.Customer.Id.Hex(),
			Uuid:       in.Customer.Uuid,
			ExternalId: in.Customer.ExternalId,
			Email:      in.Customer.Email,
			Phone:      in.Customer.Phone,
		},
		Project: &billingpb.RecurringSubscriptionProject{
			Id: in.Project.Id.Hex(),
		},
	}

	plan, err := NewRecurringPlanMapper().MapMgoToObject(in.Plan)

	if err != nil {
		return nil, err
	}

	out.Plan = plan.(*billingpb.RecurringPlan)

	projectNameLen := len(in.Project.Name)
	if projectNameLen > 0 {
		out.Project.Name = make(map[string]string, projectNameLen)

		for _, v := range in.Project.Name {
			out.Project.Name[v.Lang] = v.Value
		}
	}

	if in.LastPaymentAt != nil && !in.LastPaymentAt.IsZero() {
		out.LastPaymentAt, err = ptypes.TimestampProto(*in.LastPaymentAt)

		if err != nil {
			return nil, err
		}
	}

	if in.ExpireAt != nil && !in.ExpireAt.IsZero() {
		out.ExpireAt, err = ptypes.TimestampProto(*in.ExpireAt)

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
