package testlib

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/client"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newBotSpec() *machineidv1pb.Bot {
	return &machineidv1pb.Bot{
		Metadata: &headerv1.Metadata{
			Name: "test-bot",
		},
		Spec: &machineidv1pb.BotSpec{
			Roles: []string{"role1", "role2"},
			Traits: []*machineidv1pb.Trait{
				{
					Name:   "trait1",
					Values: []string{"value1"},
				},
			},
		},
	}
}

type botTestingPrimitives struct {
	setup *TestSetup
	reconcilers.Resource153Adapter[*machineidv1pb.Bot]
}

func (g *botTestingPrimitives) Init(setup *TestSetup) {
	g.setup = setup
}

func (g *botTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *botTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: newBotSpec(),
	})
	return trace.Wrap(err)
}

func (g *botTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*machineidv1pb.Bot, error) {
	return g.setup.TeleportClient.BotServiceClient().GetBot(ctx, &machineidv1pb.GetBotRequest{
		BotName: name,
	})
}

func (g *botTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{
		BotName: name,
	})
	return trace.Wrap(err)
}

func (g *botTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: newBotSpec(),
	})
	return trace.Wrap(err)
}

func (g *botTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{
		BotName: name,
	})
	return trace.Wrap(err)
}

func (g *botTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportBot, error) {
	bot, err := g.setup.TeleportClient.BotServiceClient().GetBot(ctx, &machineidv1pb.GetBotRequest{
		BotName: name,
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportBot := resourcesv1.TeleportBot{
		ObjectMeta: metav1.ObjectMeta{
			Name: bot.Metadata.Name,
		},
	}

	return &teleportBot, trace.Wrap(err)
}

func (g *botTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.BotServiceClient().UpdateBot(ctx, &machineidv1pb.UpdateBotRequest{
		Bot: newBotSpec(),
	})
	return trace.Wrap(err)
}

func (g *botTestingPrimitives) CompareTeleportAndKubernetesResource(tResource *machineidv1pb.Bot, kubeResource *resourcesv1.TeleportBot) (bool, string) {
	opts := CompareOptions()
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), opts...)
	return diff == "", diff
}

func TeleportBotCreationTest(t *testing.T, clt *client.Client) {
	test := &botTestingPrimitives{}
	ResourceCreationTest(t, test, WithTeleportClient(clt))
}

func TeleportBotDeletionDriftTest(t *testing.T, clt *client.Client) {
	test := &botTestingPrimitives{}
	ResourceDeletionDriftTest(t, test, WithTeleportClient(clt))
}

func TeleportBotUpdateTest(t *testing.T, clt *client.Client) {
	test := &botTestingPrimitives{}
	ResourceUpdateTest(t, test, WithTeleportClient(clt))
}
