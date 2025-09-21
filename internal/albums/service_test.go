package albums

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	service := NewService(nil)
	assert.NotNil(t, service)
}

func TestCreateAlbumRequest(t *testing.T) {
	t.Run("basic album", func(t *testing.T) {
		ownerID := uuid.New()
		req := CreateAlbumRequest{
			OwnerID:     ownerID,
			Name:        "Vacation 2024",
			Description: "Summer vacation photos",
		}

		assert.Equal(t, ownerID, req.OwnerID)
		assert.Equal(t, "Vacation 2024", req.Name)
		assert.Equal(t, "Summer vacation photos", req.Description)
	})
}

func TestUpdateAlbumRequest(t *testing.T) {
	t.Run("update basic fields", func(t *testing.T) {
		req := UpdateAlbumRequest{
			Name:        "Updated Album Name",
			Description: "Updated description",
		}

		assert.Equal(t, "Updated Album Name", req.Name)
		assert.Equal(t, "Updated description", req.Description)
		assert.Nil(t, req.ThumbnailAssetID)
	})

	t.Run("update with thumbnail", func(t *testing.T) {
		thumbnailID := uuid.New()
		req := UpdateAlbumRequest{
			Name:             "Album with Thumbnail",
			Description:      "Has a thumbnail",
			ThumbnailAssetID: &thumbnailID,
		}

		assert.Equal(t, "Album with Thumbnail", req.Name)
		assert.NotNil(t, req.ThumbnailAssetID)
		assert.Equal(t, thumbnailID, *req.ThumbnailAssetID)
	})
}

func TestAlbumInfo(t *testing.T) {
	now := time.Now()
	albumID := uuid.New()
	ownerID := uuid.New()

	album := AlbumInfo{
		ID:          albumID,
		OwnerID:     ownerID,
		Name:        "Test Album",
		Description: "Test Description",
		AssetCount:  10,
		CreatedAt:   now,
		UpdatedAt:   now,
		SharedUsers: []SharedUser{
			{
				UserID: uuid.New(),
				Role:   "editor",
			},
			{
				UserID: uuid.New(),
				Role:   "viewer",
			},
		},
		ThumbnailAssetID:  nil,
		IsActivityEnabled: true,
	}

	assert.Equal(t, albumID, album.ID)
	assert.Equal(t, ownerID, album.OwnerID)
	assert.Equal(t, "Test Album", album.Name)
	assert.Equal(t, 10, album.AssetCount)
	assert.Len(t, album.SharedUsers, 2)
	assert.Equal(t, "editor", album.SharedUsers[0].Role)
	assert.Equal(t, "viewer", album.SharedUsers[1].Role)
	assert.True(t, album.IsActivityEnabled)
}

func TestSharedUser(t *testing.T) {
	user := SharedUser{
		UserID: uuid.New(),
		Role:   "editor",
	}

	assert.NotEqual(t, uuid.Nil, user.UserID)
	assert.Equal(t, "editor", user.Role)

	t.Run("viewer role", func(t *testing.T) {
		viewer := SharedUser{
			UserID: uuid.New(),
			Role:   "viewer",
		}

		assert.NotEqual(t, uuid.Nil, viewer.UserID)
		assert.Equal(t, "viewer", viewer.Role)
	})
}

func TestAddAssetRequest(t *testing.T) {
	assetID := uuid.New()

	req := AddAssetRequest{
		AssetID: assetID,
	}

	assert.Equal(t, assetID, req.AssetID)
	assert.NotEqual(t, uuid.Nil, req.AssetID)
}

func TestRemoveAssetRequest(t *testing.T) {
	assetID := uuid.New()

	req := RemoveAssetRequest{
		AssetID: assetID,
	}

	assert.Equal(t, assetID, req.AssetID)
	assert.NotEqual(t, uuid.Nil, req.AssetID)
}

func TestShareAlbumRequest(t *testing.T) {
	t.Run("share with view permission", func(t *testing.T) {
		req := ShareAlbumRequest{
			UserID: uuid.New(),
			Role:   "viewer",
		}

		assert.NotEqual(t, uuid.Nil, req.UserID)
		assert.Equal(t, "viewer", req.Role)
	})

	t.Run("share with edit permission", func(t *testing.T) {
		req := ShareAlbumRequest{
			UserID: uuid.New(),
			Role:   "editor",
		}

		assert.NotEqual(t, uuid.Nil, req.UserID)
		assert.Equal(t, "editor", req.Role)
	})
}

func TestUnshareAlbumRequest(t *testing.T) {
	userID := uuid.New()
	req := UnshareAlbumRequest{
		UserID: userID,
	}

	assert.Equal(t, userID, req.UserID)
	assert.NotEqual(t, uuid.Nil, req.UserID)
}

func TestAlbumListResponse(t *testing.T) {
	albums := []AlbumInfo{
		{
			ID:          uuid.New(),
			OwnerID:     uuid.New(),
			Name:        "Album 1",
			Description: "First album",
			AssetCount:  5,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			OwnerID:     uuid.New(),
			Name:        "Album 2",
			Description: "Second album",
			AssetCount:  10,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	response := AlbumListResponse{
		Albums: albums,
		Total:  2,
	}

	assert.Len(t, response.Albums, 2)
	assert.Equal(t, int64(2), response.Total)
	assert.Equal(t, "Album 1", response.Albums[0].Name)
	assert.Equal(t, "Album 2", response.Albums[1].Name)
	assert.Equal(t, 5, response.Albums[0].AssetCount)
	assert.Equal(t, 10, response.Albums[1].AssetCount)
}

func TestAlbumActivity(t *testing.T) {
	activityTime := time.Now()
	activity := AlbumActivity{
		ID:        uuid.New(),
		AlbumID:   uuid.New(),
		UserID:    uuid.New(),
		AssetID:   nil,
		Comment:   "Great photos!",
		IsLiked:   false,
		CreatedAt: activityTime,
		UpdatedAt: activityTime,
	}

	assert.NotEqual(t, uuid.Nil, activity.ID)
	assert.NotEqual(t, uuid.Nil, activity.AlbumID)
	assert.NotEqual(t, uuid.Nil, activity.UserID)
	assert.Nil(t, activity.AssetID)
	assert.Equal(t, "Great photos!", activity.Comment)
	assert.False(t, activity.IsLiked)

	t.Run("activity with asset and like", func(t *testing.T) {
		assetID := uuid.New()

		assetActivity := AlbumActivity{
			ID:        uuid.New(),
			AlbumID:   uuid.New(),
			UserID:    uuid.New(),
			AssetID:   &assetID,
			Comment:   "",
			IsLiked:   true,
			CreatedAt: activityTime,
			UpdatedAt: activityTime,
		}

		assert.NotNil(t, assetActivity.AssetID)
		assert.Equal(t, assetID, *assetActivity.AssetID)
		assert.Empty(t, assetActivity.Comment)
		assert.True(t, assetActivity.IsLiked)
	})
}

func TestAlbumStatistics(t *testing.T) {
	stats := AlbumStatistics{
		TotalAlbums:       50,
		TotalSharedAlbums: 15,
		TotalAssets:       1500,
	}

	assert.Equal(t, int64(50), stats.TotalAlbums)
	assert.Equal(t, int64(15), stats.TotalSharedAlbums)
	assert.Equal(t, int64(1500), stats.TotalAssets)

	// Test that shared albums is less than or equal to total albums
	assert.LessOrEqual(t, stats.TotalSharedAlbums, stats.TotalAlbums)
}

func TestAlbumWithThumbnail(t *testing.T) {
	thumbnailID := uuid.New()

	album := AlbumInfo{
		ID:                uuid.New(),
		OwnerID:           uuid.New(),
		Name:              "Album with Thumbnail",
		Description:       "Has a thumbnail",
		AssetCount:        100,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		SharedUsers:       []SharedUser{},
		ThumbnailAssetID:  &thumbnailID,
		IsActivityEnabled: true,
	}

	assert.NotNil(t, album.ThumbnailAssetID)
	assert.Equal(t, thumbnailID, *album.ThumbnailAssetID)
	assert.True(t, album.IsActivityEnabled)
}

func TestContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate some operation
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	select {
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have been cancelled")
	}
}

func TestAlbum_IsShared(t *testing.T) {
	t.Run("not shared", func(t *testing.T) {
		album := AlbumInfo{
			ID:          uuid.New(),
			OwnerID:     uuid.New(),
			Name:        "Private Album",
			SharedUsers: []SharedUser{},
		}

		assert.Empty(t, album.SharedUsers)
	})

	t.Run("shared with users", func(t *testing.T) {
		album := AlbumInfo{
			ID:      uuid.New(),
			OwnerID: uuid.New(),
			Name:    "Shared Album",
			SharedUsers: []SharedUser{
				{UserID: uuid.New(), Role: "viewer"},
			},
		}

		assert.NotEmpty(t, album.SharedUsers)
		assert.Len(t, album.SharedUsers, 1)
	})
}