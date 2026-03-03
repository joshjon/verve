package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/postgres"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/task"
)

func TestDeleteRepo_CascadeDeletesAllRelatedRows(t *testing.T) {
	pool := postgres.NewTestDB(t)
	repoRepo := postgres.NewRepoRepository(pool)
	taskRepo := postgres.NewTaskRepository(pool)
	epicRepo := postgres.NewEpicRepository(pool)

	ctx := context.Background()

	// Create a repo.
	r, err := repo.NewRepo("owner/cascade-test")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(ctx, r))

	// Create an epic for the repo.
	e := epic.NewEpic(r.ID.String(), "Test Epic", "Epic description")
	require.NoError(t, epicRepo.CreateEpic(ctx, e))

	// Create a task for the repo (associated with the epic).
	tsk := task.NewTask(r.ID.String(), "Test Task", "Task description", nil, nil, 0, false, false, "", true)
	tsk.EpicID = e.ID.String()
	require.NoError(t, taskRepo.CreateTask(ctx, tsk))

	// Create a task not associated with any epic.
	tsk2 := task.NewTask(r.ID.String(), "Standalone Task", "No epic", nil, nil, 0, false, false, "", true)
	require.NoError(t, taskRepo.CreateTask(ctx, tsk2))

	// Append logs for both tasks.
	require.NoError(t, taskRepo.AppendTaskLogs(ctx, tsk.ID, 1, []string{"log line 1", "log line 2"}))
	require.NoError(t, taskRepo.AppendTaskLogs(ctx, tsk2.ID, 1, []string{"log line 3"}))

	// Verify everything exists before deletion.
	_, err = repoRepo.ReadRepo(ctx, r.ID)
	require.NoError(t, err)

	_, err = epicRepo.ReadEpic(ctx, e.ID)
	require.NoError(t, err)

	_, err = taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)

	_, err = taskRepo.ReadTask(ctx, tsk2.ID)
	require.NoError(t, err)

	logs, err := taskRepo.ReadTaskLogs(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	// Delete the repo — should cascade to epics, tasks, and task_logs.
	err = repoRepo.DeleteRepo(ctx, r.ID)
	require.NoError(t, err, "deleting repo with tasks and epics should succeed via ON DELETE CASCADE")

	// Verify repo is gone.
	_, err = repoRepo.ReadRepo(ctx, r.ID)
	assert.Error(t, err, "repo should be deleted")

	// Verify epic is gone.
	_, err = epicRepo.ReadEpic(ctx, e.ID)
	assert.Error(t, err, "epic should be cascade deleted")

	// Verify tasks are gone.
	_, err = taskRepo.ReadTask(ctx, tsk.ID)
	assert.Error(t, err, "task should be cascade deleted")

	_, err = taskRepo.ReadTask(ctx, tsk2.ID)
	assert.Error(t, err, "standalone task should be cascade deleted")

	// Verify task logs are gone.
	logs, err = taskRepo.ReadTaskLogs(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Empty(t, logs, "task logs should be cascade deleted")

	logs2, err := taskRepo.ReadTaskLogs(ctx, tsk2.ID)
	require.NoError(t, err)
	assert.Empty(t, logs2, "standalone task logs should be cascade deleted")
}

func TestDeleteRepo_DoesNotAffectOtherRepos(t *testing.T) {
	pool := postgres.NewTestDB(t)
	repoRepo := postgres.NewRepoRepository(pool)
	taskRepo := postgres.NewTaskRepository(pool)
	epicRepo := postgres.NewEpicRepository(pool)

	ctx := context.Background()

	// Create two repos.
	r1, err := repo.NewRepo("owner/repo1")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(ctx, r1))

	r2, err := repo.NewRepo("owner/repo2")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(ctx, r2))

	// Create data for both repos.
	e1 := epic.NewEpic(r1.ID.String(), "Epic 1", "desc")
	require.NoError(t, epicRepo.CreateEpic(ctx, e1))

	e2 := epic.NewEpic(r2.ID.String(), "Epic 2", "desc")
	require.NoError(t, epicRepo.CreateEpic(ctx, e2))

	tsk1 := task.NewTask(r1.ID.String(), "Task 1", "desc", nil, nil, 0, false, false, "", true)
	require.NoError(t, taskRepo.CreateTask(ctx, tsk1))

	tsk2 := task.NewTask(r2.ID.String(), "Task 2", "desc", nil, nil, 0, false, false, "", true)
	require.NoError(t, taskRepo.CreateTask(ctx, tsk2))

	require.NoError(t, taskRepo.AppendTaskLogs(ctx, tsk1.ID, 1, []string{"r1 log"}))
	require.NoError(t, taskRepo.AppendTaskLogs(ctx, tsk2.ID, 1, []string{"r2 log"}))

	// Delete only repo1.
	err = repoRepo.DeleteRepo(ctx, r1.ID)
	require.NoError(t, err)

	// Repo2 and its data should still exist.
	_, err = repoRepo.ReadRepo(ctx, r2.ID)
	assert.NoError(t, err, "repo2 should not be affected")

	_, err = epicRepo.ReadEpic(ctx, e2.ID)
	assert.NoError(t, err, "epic2 should not be affected")

	_, err = taskRepo.ReadTask(ctx, tsk2.ID)
	assert.NoError(t, err, "task2 should not be affected")

	logs, err := taskRepo.ReadTaskLogs(ctx, tsk2.ID)
	require.NoError(t, err)
	assert.Len(t, logs, 1, "task2 logs should not be affected")

	// Repo1 data should be gone.
	_, err = repoRepo.ReadRepo(ctx, r1.ID)
	assert.Error(t, err, "repo1 should be deleted")

	_, err = epicRepo.ReadEpic(ctx, e1.ID)
	assert.Error(t, err, "epic1 should be cascade deleted")

	_, err = taskRepo.ReadTask(ctx, tsk1.ID)
	assert.Error(t, err, "task1 should be cascade deleted")
}

func TestDeleteRepo_EpicDeleteSetsTaskEpicIDToNull(t *testing.T) {
	pool := postgres.NewTestDB(t)
	repoRepo := postgres.NewRepoRepository(pool)
	taskRepo := postgres.NewTaskRepository(pool)
	epicRepo := postgres.NewEpicRepository(pool)

	ctx := context.Background()

	// Create a repo and an epic.
	r, err := repo.NewRepo("owner/epic-null-test")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(ctx, r))

	e := epic.NewEpic(r.ID.String(), "Epic", "desc")
	require.NoError(t, epicRepo.CreateEpic(ctx, e))

	// Create a task linked to the epic.
	tsk := task.NewTask(r.ID.String(), "Task", "desc", nil, nil, 0, false, false, "", true)
	tsk.EpicID = e.ID.String()
	require.NoError(t, taskRepo.CreateTask(ctx, tsk))

	// Delete just the epic (not the repo). Task should have epic_id set to NULL.
	err = epicRepo.DeleteEpic(ctx, e.ID)
	require.NoError(t, err)

	got, err := taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Empty(t, got.EpicID, "task epic_id should be NULL after epic deletion")
}
