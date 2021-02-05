package repository

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/repository/models"
	"github.com/paysuper/paysuper-proto/go/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	mongodb "gopkg.in/paysuper/paysuper-database-mongo.v2"
	"testing"
)

type RecurringPlanTestSuite struct {
	suite.Suite
	db         mongodb.SourceInterface
	repository *recurringPlanRepository
	log        *zap.Logger
}

func Test_RecurringPlan(t *testing.T) {
	suite.Run(t, new(RecurringPlanTestSuite))
}

func (suite *RecurringPlanTestSuite) SetupTest() {
	_, err := config.NewConfig()
	assert.NoError(suite.T(), err, "Config load failed")

	suite.log, err = zap.NewProduction()
	assert.NoError(suite.T(), err, "Logger initialization failed")

	suite.db, err = mongodb.NewDatabase()
	assert.NoError(suite.T(), err, "Database connection failed")

	suite.repository = &recurringPlanRepository{db: suite.db, mapper: models.NewRecurringPlanMapper()}
}

func (suite *RecurringPlanTestSuite) TearDownTest() {
	if err := suite.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	if err := suite.db.Close(); err != nil {
		suite.FailNow("Database close failed", "%v", err)
	}
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_NewRecurringPlanRepository_Ok() {
	repository := NewRecurringPlanRepository(suite.db)
	assert.IsType(suite.T(), &recurringPlanRepository{}, repository)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_Insert() {
	plan := suite.template()

	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	plan2, err := suite.repository.GetById(context.TODO(), plan.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), plan.Id, plan2.Id)
	assert.Equal(suite.T(), plan.MerchantId, plan2.MerchantId)
	assert.Equal(suite.T(), plan.ProjectId, plan2.ProjectId)
	assert.Equal(suite.T(), plan.Name, plan2.Name)
	assert.Equal(suite.T(), plan.Description, plan2.Description)
	assert.Equal(suite.T(), plan.ExternalId, plan2.ExternalId)
	assert.Equal(suite.T(), plan.GroupId, plan2.GroupId)
	assert.Equal(suite.T(), plan.Tags, plan2.Tags)
	assert.Equal(suite.T(), plan.Status, plan2.Status)
	assert.Equal(suite.T(), plan.GracePeriod, plan2.GracePeriod)
	assert.Equal(suite.T(), plan.Expiration, plan2.Expiration)
	assert.Equal(suite.T(), plan.Trial, plan2.Trial)
	assert.Equal(suite.T(), plan.GracePeriod, plan2.GracePeriod)
	assert.NotEmpty(suite.T(), plan2.CreatedAt)
	assert.NotEmpty(suite.T(), plan2.UpdatedAt)
	assert.Empty(suite.T(), plan2.DeletedAt)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_Update() {
	plan := suite.template()

	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	plan2, err := suite.repository.GetById(context.TODO(), plan.Id)
	assert.NoError(suite.T(), err)

	plan2.MerchantId = primitive.NewObjectID().Hex()
	plan2.ProjectId = primitive.NewObjectID().Hex()
	plan2.Name = map[string]string{"en": "name2"}
	plan2.Description = map[string]string{"en": "description2"}
	plan2.ExternalId = "external_id2"
	plan2.GroupId = "group_id2"
	plan2.Tags = []string{"tag2"}
	plan2.Status = "disabled"
	plan2.Charge = &billingpb.RecurringPlanCharge{
		Period: &billingpb.RecurringPlanPeriod{
			Value: 20,
			Type:  "month",
		},
		Amount:   100,
		Currency: "RUB",
	}
	plan2.Expiration = &billingpb.RecurringPlanPeriod{
		Value: 10,
		Type:  "month",
	}
	plan2.Trial = &billingpb.RecurringPlanPeriod{
		Value: 11,
		Type:  "month",
	}
	plan2.GracePeriod = &billingpb.RecurringPlanPeriod{
		Value: 12,
		Type:  "month",
	}

	err = suite.repository.Update(context.TODO(), plan2)
	assert.NoError(suite.T(), err)

	plan3, err := suite.repository.GetById(context.TODO(), plan.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), plan.Id, plan3.Id)
	assert.Equal(suite.T(), plan2.MerchantId, plan3.MerchantId)
	assert.Equal(suite.T(), plan2.ProjectId, plan3.ProjectId)
	assert.Equal(suite.T(), plan2.Name, plan3.Name)
	assert.Equal(suite.T(), plan2.Description, plan3.Description)
	assert.Equal(suite.T(), plan2.ExternalId, plan3.ExternalId)
	assert.Equal(suite.T(), plan2.GroupId, plan3.GroupId)
	assert.Equal(suite.T(), plan2.Tags, plan3.Tags)
	assert.Equal(suite.T(), plan2.Status, plan3.Status)
	assert.Equal(suite.T(), plan2.GracePeriod, plan3.GracePeriod)
	assert.Equal(suite.T(), plan2.Expiration, plan3.Expiration)
	assert.Equal(suite.T(), plan2.Trial, plan3.Trial)
	assert.Equal(suite.T(), plan2.GracePeriod, plan3.GracePeriod)
	assert.NotEmpty(suite.T(), plan3.CreatedAt)
	assert.NotEmpty(suite.T(), plan3.UpdatedAt)
	assert.Empty(suite.T(), plan3.DeletedAt)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_GetByIdDeleted() {
	plan := suite.template()
	plan.DeletedAt = ptypes.TimestampNow()

	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	_, err = suite.repository.GetById(context.TODO(), plan.Id)
	assert.Error(suite.T(), err)
	assert.EqualError(suite.T(), err, mongo.ErrNoDocuments.Error())
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_FindError_RequireProjectId() {
	plan := suite.template()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	_, err = suite.repository.Find(context.TODO(), plan.MerchantId, "", "", "", "", 0, 1)
	assert.Error(suite.T(), err)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_FindError_RequireMerchantId() {
	plan := suite.template()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	_, err = suite.repository.Find(context.TODO(), "", plan.ProjectId, "", "", "", 0, 1)
	assert.Error(suite.T(), err)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_Find() {
	plan := suite.template()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	plan2 := suite.template()
	err = suite.repository.Insert(context.TODO(), plan2)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.Find(context.TODO(), plan.MerchantId, plan.ProjectId, "", "", "", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), plan.Id, list[0].Id)

	cnt, err := suite.repository.FindCount(context.TODO(), plan.MerchantId, plan.ProjectId, "", "", "")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), cnt)

	list, err = suite.repository.Find(context.TODO(), plan2.MerchantId, plan2.ProjectId, "", "", "", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), plan2.Id, list[0].Id)

	cnt, err = suite.repository.FindCount(context.TODO(), plan2.MerchantId, plan2.ProjectId, "", "", "")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), cnt)

	list, err = suite.repository.Find(context.TODO(), primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex(), "", "", "", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 0)

	cnt, err = suite.repository.FindCount(context.TODO(), primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex(), "", "", "")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), cnt)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_FindByExternalId() {
	plan := suite.template()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	plan2 := plan
	plan2.Id = primitive.NewObjectID().Hex()
	plan2.ExternalId = "ext2"
	err = suite.repository.Insert(context.TODO(), plan2)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.Find(context.TODO(), plan2.MerchantId, plan2.ProjectId, plan2.ExternalId, "", "", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), plan2.Id, list[0].Id)
	assert.Equal(suite.T(), plan2.ExternalId, list[0].ExternalId)

	cnt, err := suite.repository.FindCount(context.TODO(), plan2.MerchantId, plan2.ProjectId, plan2.ExternalId, "", "")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), cnt)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_FindByGroupId() {
	plan := suite.template()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	plan2 := plan
	plan2.Id = primitive.NewObjectID().Hex()
	plan2.GroupId = "group2"
	err = suite.repository.Insert(context.TODO(), plan2)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.Find(context.TODO(), plan2.MerchantId, plan2.ProjectId, "", plan2.GroupId, "", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), plan2.Id, list[0].Id)
	assert.Equal(suite.T(), plan2.GroupId, list[0].GroupId)

	cnt, err := suite.repository.FindCount(context.TODO(), plan2.MerchantId, plan2.ProjectId, "", plan2.GroupId, "")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), cnt)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_FindByQuery() {
	plan := suite.template()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	plan2 := plan
	plan2.Id = primitive.NewObjectID().Hex()
	plan2.Name = map[string]string{"en": "name2"}
	err = suite.repository.Insert(context.TODO(), plan2)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.Find(context.TODO(), plan2.MerchantId, plan2.ProjectId, "", "", "name2", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), plan2.Id, list[0].Id)
	assert.Equal(suite.T(), plan2.Name, list[0].Name)

	cnt, err := suite.repository.FindCount(context.TODO(), plan2.MerchantId, plan2.ProjectId, "", "", "name2")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), cnt)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_FindOffset() {
	plan := suite.template()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	plan2 := suite.template()
	plan2.MerchantId = primitive.NewObjectID().Hex()
	plan2.MerchantId = plan.MerchantId
	plan2.ProjectId = plan.ProjectId
	err = suite.repository.Insert(context.TODO(), plan2)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.Find(context.TODO(), plan.MerchantId, plan.ProjectId, "", "", "", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), plan.Id, list[0].Id)

	list, err = suite.repository.Find(context.TODO(), plan.MerchantId, plan.ProjectId, "", "", "", 1, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
	assert.Equal(suite.T(), plan2.Id, list[0].Id)
}

func (suite *RecurringPlanTestSuite) TestRecurringPlan_FindDeleted() {
	plan := suite.template()
	plan.DeletedAt = ptypes.TimestampNow()
	err := suite.repository.Insert(context.TODO(), plan)
	assert.NoError(suite.T(), err)

	list, err := suite.repository.Find(context.TODO(), plan.MerchantId, plan.ProjectId, "", "", "", 0, 1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 0)

	count, err := suite.repository.FindCount(context.TODO(), plan.MerchantId, plan.ProjectId, "", "", "")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)
}

func (suite *RecurringPlanTestSuite) template() *billingpb.RecurringPlan {
	return &billingpb.RecurringPlan{
		Id:          primitive.NewObjectID().Hex(),
		MerchantId:  primitive.NewObjectID().Hex(),
		ProjectId:   primitive.NewObjectID().Hex(),
		Name:        map[string]string{"en": "name"},
		Description: map[string]string{"en": "description"},
		ExternalId:  "external_id",
		GroupId:     "group_id",
		Charge: &billingpb.RecurringPlanCharge{
			Period: &billingpb.RecurringPlanPeriod{
				Value: 7,
				Type:  "day",
			},
			Amount:   10,
			Currency: "USD",
		},
		Expiration: &billingpb.RecurringPlanPeriod{
			Value: 6,
			Type:  "day",
		},
		Trial: &billingpb.RecurringPlanPeriod{
			Value: 5,
			Type:  "day",
		},
		GracePeriod: &billingpb.RecurringPlanPeriod{
			Value: 4,
			Type:  "day",
		},
		Status: "active",
		Tags:   []string{"tag"},
	}
}
