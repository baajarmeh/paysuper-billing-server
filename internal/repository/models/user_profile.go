package models

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type userProfileMapper struct{}

func NewUserProfileMapper() Mapper {
	return &userProfileMapper{}
}

type MgoUserProfile struct {
	Id        primitive.ObjectID             `bson:"_id" faker:"objectId"`
	UserId    string                         `bson:"user_id"`
	Email     *MgoUserProfileEmail           `bson:"email"`
	Personal  *billingpb.UserProfilePersonal `bson:"personal"`
	Help      *billingpb.UserProfileHelp     `bson:"help"`
	Company   *billingpb.UserProfileCompany  `bson:"company"`
	LastStep  string                         `bson:"last_step"`
	Locale    string                         `bson:"locale"`
	CreatedAt time.Time                      `bson:"created_at"`
	UpdatedAt time.Time                      `bson:"updated_at"`
}

type MgoUserProfileEmail struct {
	Email                   string    `bson:"email"`
	Confirmed               bool      `bson:"confirmed"`
	ConfirmedAt             time.Time `bson:"confirmed_at"`
	IsConfirmationEmailSent bool      `bson:"is_confirmation_email_sent"`
}

func (m *userProfileMapper) MapObjectToMgo(obj interface{}) (interface{}, error) {
	in := obj.(*billingpb.UserProfile)

	out := &MgoUserProfile{
		UserId:   in.UserId,
		Personal: in.Personal,
		Help:     in.Help,
		Company:  in.Company,
		LastStep: in.LastStep,
		Locale:   in.Locale,
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

	if in.Email != nil {
		out.Email = &MgoUserProfileEmail{
			Email:                   in.Email.Email,
			Confirmed:               in.Email.Confirmed,
			IsConfirmationEmailSent: in.Email.IsConfirmationEmailSent,
		}

		if in.Email.ConfirmedAt != nil {
			t, err := ptypes.Timestamp(in.Email.ConfirmedAt)

			if err != nil {
				return nil, err
			}

			out.Email.ConfirmedAt = t
		}
	}

	return out, nil
}

func (m *userProfileMapper) MapMgoToObject(obj interface{}) (interface{}, error) {
	var err error
	in := obj.(*MgoUserProfile)

	out := &billingpb.UserProfile{
		Id:       in.Id.Hex(),
		UserId:   in.UserId,
		Personal: in.Personal,
		Help:     in.Help,
		Company:  in.Company,
		LastStep: in.LastStep,
		Locale:   in.Locale,
	}

	out.CreatedAt, err = ptypes.TimestampProto(in.CreatedAt)

	if err != nil {
		return nil, err
	}

	out.UpdatedAt, err = ptypes.TimestampProto(in.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if in.Email != nil {
		out.Email = &billingpb.UserProfileEmail{
			Email:                   in.Email.Email,
			Confirmed:               in.Email.Confirmed,
			IsConfirmationEmailSent: in.Email.IsConfirmationEmailSent,
		}

		if !in.Email.ConfirmedAt.IsZero() {
			out.Email.ConfirmedAt, err = ptypes.TimestampProto(in.Email.ConfirmedAt)

			if err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}
