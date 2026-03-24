package auction

import (
	"context"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestAuctionAutoClose(t *testing.T) {
	mongoURL := os.Getenv("MONGODB_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://admin:admin@localhost:27017/auctions?authSource=admin"
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB not reachable: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("test_auction_autoclose")
	defer db.Drop(ctx)

	repo := NewAuctionRepository(db)

	os.Setenv("AUCTION_DURATION", "2s")
	defer os.Unsetenv("AUCTION_DURATION")

	auctionEntity, internalErr := auction_entity.CreateAuction(
		"Test Product",
		"Electronics",
		"Test Description with enough characters",
		auction_entity.New,
	)
	if internalErr != nil {
		t.Fatalf("failed to create auction entity: %v", internalErr)
	}

	internalErr = repo.CreateAuction(ctx, auctionEntity)
	if internalErr != nil {
		t.Fatalf("failed to insert auction: %v", internalErr)
	}

	// Verify auction is Active immediately after creation
	created, internalErr := repo.FindAuctionById(ctx, auctionEntity.Id)
	if internalErr != nil {
		t.Fatalf("failed to find auction: %v", internalErr)
	}
	if created.Status != auction_entity.Active {
		t.Errorf("expected status Active, got %v", created.Status)
	}

	// Wait for the goroutine to close the auction (2s duration + 1s margin)
	time.Sleep(3 * time.Second)

	closed, internalErr := repo.FindAuctionById(ctx, auctionEntity.Id)
	if internalErr != nil {
		t.Fatalf("failed to find auction after duration: %v", internalErr)
	}
	if closed.Status != auction_entity.Completed {
		t.Errorf("expected status Completed after duration, got %v", closed.Status)
	}
}
