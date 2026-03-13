package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vervesh/verve/internal/conversation"
	"github.com/vervesh/verve/internal/epic"
	"github.com/vervesh/verve/internal/repo"
	"github.com/vervesh/verve/internal/sqlite"
)

func seedConvRepo(t *testing.T, repoRepo *sqlite.RepoRepository) *repo.Repo {
	t.Helper()
	ctx := context.Background()
	r, err := repo.NewRepo("owner/conv-test")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(ctx, r))
	return r
}

func TestConversationCRUD(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	// Create a conversation.
	conv := conversation.NewConversation(r.ID.String(), "Test Conversation", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	// Read it back.
	got, err := convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, got.ID)
	assert.Equal(t, r.ID.String(), got.RepoID)
	assert.Equal(t, "Test Conversation", got.Title)
	assert.Equal(t, conversation.StatusActive, got.Status)
	assert.Equal(t, "claude-opus-4", got.Model)
	assert.Empty(t, got.Messages)
	assert.Nil(t, got.PendingMessage)
	assert.Nil(t, got.EpicID)
	assert.Nil(t, got.ClaimedAt)
	assert.Nil(t, got.LastHeartbeatAt)

	// List by repo.
	convos, err := convRepo.ListConversationsByRepo(ctx, r.ID.String())
	require.NoError(t, err)
	assert.Len(t, convos, 1)
	assert.Equal(t, conv.ID, convos[0].ID)

	// Update status.
	require.NoError(t, convRepo.UpdateConversationStatus(ctx, conv.ID, conversation.StatusArchived))
	got, err = convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.Equal(t, conversation.StatusArchived, got.Status)

	// Delete.
	require.NoError(t, convRepo.DeleteConversation(ctx, conv.ID))
	_, err = convRepo.ReadConversation(ctx, conv.ID)
	assert.Error(t, err)
}

func TestConversation_AppendMessage(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	conv := conversation.NewConversation(r.ID.String(), "Msg Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	// Append first message.
	msg1 := conversation.Message{
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now().Unix(),
	}
	require.NoError(t, convRepo.AppendMessage(ctx, conv.ID, msg1))

	got, err := convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "Hello", got.Messages[0].Content)

	// Append second message.
	msg2 := conversation.Message{
		Role:      "assistant",
		Content:   "Hi there!",
		Timestamp: time.Now().Unix(),
	}
	require.NoError(t, convRepo.AppendMessage(ctx, conv.ID, msg2))

	got, err = convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, "assistant", got.Messages[1].Role)
	assert.Equal(t, "Hi there!", got.Messages[1].Content)
}

func TestConversation_PendingMessageFlow(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	conv := conversation.NewConversation(r.ID.String(), "Pending Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	// Initially no pending conversations.
	pending, err := convRepo.ListPendingConversations(ctx)
	require.NoError(t, err)
	assert.Empty(t, pending)

	// Set pending message.
	msg := "What is Go?"
	require.NoError(t, convRepo.SetPendingMessage(ctx, conv.ID, &msg))

	// Now should appear in pending list.
	pending, err = convRepo.ListPendingConversations(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, conv.ID, pending[0].ID)
	assert.Equal(t, &msg, pending[0].PendingMessage)

	// Clear pending message.
	require.NoError(t, convRepo.SetPendingMessage(ctx, conv.ID, nil))

	pending, err = convRepo.ListPendingConversations(ctx)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestConversation_ClaimAndRelease(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	conv := conversation.NewConversation(r.ID.String(), "Claim Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	// Set pending message so it can be claimed.
	msg := "test message"
	require.NoError(t, convRepo.SetPendingMessage(ctx, conv.ID, &msg))

	// Claim.
	ok, err := convRepo.ClaimConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	// Verify claimed_at is set.
	got, err := convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.NotNil(t, got.ClaimedAt)
	assert.NotNil(t, got.LastHeartbeatAt)

	// Can't claim again (already claimed).
	ok, err = convRepo.ClaimConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	// Should not appear in pending list while claimed.
	pending, err := convRepo.ListPendingConversations(ctx)
	require.NoError(t, err)
	assert.Empty(t, pending)

	// Release claim.
	require.NoError(t, convRepo.ReleaseConversationClaim(ctx, conv.ID))

	got, err = convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.Nil(t, got.ClaimedAt)
	assert.Nil(t, got.LastHeartbeatAt)

	// Now should appear in pending list again (still has pending message).
	pending, err = convRepo.ListPendingConversations(ctx)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
}

func TestConversation_Heartbeat(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	conv := conversation.NewConversation(r.ID.String(), "Heartbeat Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	msg := "test"
	require.NoError(t, convRepo.SetPendingMessage(ctx, conv.ID, &msg))

	ok, err := convRepo.ClaimConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	// Get initial heartbeat time.
	got, err := convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	initialHB := got.LastHeartbeatAt

	// Wait a bit and send heartbeat.
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, convRepo.ConversationHeartbeat(ctx, conv.ID))

	got, err = convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.True(t, got.LastHeartbeatAt.After(*initialHB) || got.LastHeartbeatAt.Equal(*initialHB))
}

func TestConversation_ListStale(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	conv := conversation.NewConversation(r.ID.String(), "Stale Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	msg := "test"
	require.NoError(t, convRepo.SetPendingMessage(ctx, conv.ID, &msg))

	ok, err := convRepo.ClaimConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	// With a future threshold, all claimed conversations should be stale.
	stale, err := convRepo.ListStaleConversations(ctx, time.Now().Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, stale, 1)

	// With a past threshold, none should be stale.
	stale, err = convRepo.ListStaleConversations(ctx, time.Now().Add(-1*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, stale)
}

func TestConversation_SetEpicID(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	epicRepo := sqlite.NewEpicRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	// Create an epic for linking.
	e := epic.NewEpic(r.ID.String(), "Test Epic", "desc")
	require.NoError(t, epicRepo.CreateEpic(ctx, e))

	conv := conversation.NewConversation(r.ID.String(), "Epic Link Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	// Set epic ID.
	require.NoError(t, convRepo.SetEpicID(ctx, conv.ID, e.ID.String()))

	got, err := convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	require.NotNil(t, got.EpicID)
	assert.Equal(t, e.ID.String(), *got.EpicID)
}

func TestConversation_ListActiveConversations(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	// Create an active and an archived conversation.
	active := conversation.NewConversation(r.ID.String(), "Active", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, active))

	archived := conversation.NewConversation(r.ID.String(), "Archived", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, archived))
	require.NoError(t, convRepo.UpdateConversationStatus(ctx, archived.ID, conversation.StatusArchived))

	convos, err := convRepo.ListActiveConversations(ctx)
	require.NoError(t, err)
	assert.Len(t, convos, 1)
	assert.Equal(t, active.ID, convos[0].ID)
}

func TestConversation_NotFoundError(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)

	ctx := context.Background()
	fakeID := conversation.NewConversationID()

	_, err := convRepo.ReadConversation(ctx, fakeID)
	assert.Error(t, err)
}

func TestConversation_CascadeDeleteOnRepoDelete(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	conv := conversation.NewConversation(r.ID.String(), "Cascade Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))

	// Verify it exists.
	_, err := convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)

	// Delete the repo — should cascade delete the conversation.
	require.NoError(t, repoRepo.DeleteRepo(ctx, r.ID))

	_, err = convRepo.ReadConversation(ctx, conv.ID)
	assert.Error(t, err, "conversation should be cascade deleted when repo is deleted")
}

func TestConversation_EpicSetNullOnDelete(t *testing.T) {
	db := sqlite.NewTestDB(t)
	convRepo := sqlite.NewConversationRepository(db)
	repoRepo := sqlite.NewRepoRepository(db)
	epicRepo := sqlite.NewEpicRepository(db)
	ctx := context.Background()

	r := seedConvRepo(t, repoRepo)

	e := epic.NewEpic(r.ID.String(), "Epic", "desc")
	require.NoError(t, epicRepo.CreateEpic(ctx, e))

	conv := conversation.NewConversation(r.ID.String(), "Epic Null Test", "claude-opus-4")
	require.NoError(t, convRepo.CreateConversation(ctx, conv))
	require.NoError(t, convRepo.SetEpicID(ctx, conv.ID, e.ID.String()))

	// Delete the epic — conversation epic_id should become NULL.
	require.NoError(t, epicRepo.DeleteEpic(ctx, e.ID))

	got, err := convRepo.ReadConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.Nil(t, got.EpicID, "epic_id should be NULL after epic deletion")
}
