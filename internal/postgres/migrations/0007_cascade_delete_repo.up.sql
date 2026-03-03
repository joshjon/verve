-- Add ON DELETE CASCADE to all foreign keys that reference repo, epic, and task.
-- This ensures deleting a repo cascades to epics, tasks, and task_logs.

-- task_log.task_id → task.id
ALTER TABLE task_log DROP CONSTRAINT task_log_task_id_fkey;
ALTER TABLE task_log ADD CONSTRAINT task_log_task_id_fkey
    FOREIGN KEY (task_id) REFERENCES task (id) ON DELETE CASCADE;

-- task.epic_id → epic.id
ALTER TABLE task DROP CONSTRAINT task_epic_id_fkey;
ALTER TABLE task ADD CONSTRAINT task_epic_id_fkey
    FOREIGN KEY (epic_id) REFERENCES epic (id) ON DELETE SET NULL;

-- task.repo_id → repo.id
ALTER TABLE task DROP CONSTRAINT task_repo_id_fkey;
ALTER TABLE task ADD CONSTRAINT task_repo_id_fkey
    FOREIGN KEY (repo_id) REFERENCES repo (id) ON DELETE CASCADE;

-- epic.repo_id → repo.id
ALTER TABLE epic DROP CONSTRAINT epic_repo_id_fkey;
ALTER TABLE epic ADD CONSTRAINT epic_repo_id_fkey
    FOREIGN KEY (repo_id) REFERENCES repo (id) ON DELETE CASCADE;
