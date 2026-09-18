package migration

// AllMigrations returns all registered migrations in order.
// Add new migrations here as the schema evolves.
func AllMigrations() []Migration {
	return []Migration{
		migration001CreateUsersTable(),
		migration002CreateTasksTable(),
		migration003CreateTaskLogsTable(),
		migration004CreateIdempotencyRecordsTable(),
		migration005AddIndexes(),
	}
}

// ---------------------------------------------------------------
// 001 - Create users table
// ---------------------------------------------------------------
func migration001CreateUsersTable() Migration {
	return Migration{
		Version:     1,
		Description: "Create users table",
		Up: `
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";

		CREATE TABLE IF NOT EXISTS users (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username   VARCHAR(255) NOT NULL UNIQUE,
			password   VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);`,
		Down: `DROP TABLE IF EXISTS users;`,
	}
}

// ---------------------------------------------------------------
// 002 - Create tasks table
// ---------------------------------------------------------------
func migration002CreateTasksTable() Migration {
	return Migration{
		Version:     2,
		Description: "Create tasks table",
		Up: `
		CREATE TABLE IF NOT EXISTS tasks (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title       VARCHAR(255) NOT NULL,
			description TEXT DEFAULT '',
			status      VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'in-progress', 'completed')),
			assignee_id UUID NOT NULL,
			created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_tasks_assignee FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		Down: `DROP TABLE IF EXISTS tasks;`,
	}
}

// ---------------------------------------------------------------
// 003 - Create task_logs table
// ---------------------------------------------------------------
func migration003CreateTaskLogsTable() Migration {
	return Migration{
		Version:     3,
		Description: "Create task_logs table",
		Up: `
		CREATE TABLE IF NOT EXISTS task_logs (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			task_id    UUID NOT NULL,
			action     VARCHAR(50) NOT NULL,
			old_value  TEXT DEFAULT '',
			new_value  TEXT DEFAULT '',
			changed_by UUID NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_task_logs_task      FOREIGN KEY (task_id)    REFERENCES tasks(id) ON DELETE CASCADE,
			CONSTRAINT fk_task_logs_changed_by FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
		);`,
		Down: `DROP TABLE IF EXISTS task_logs;`,
	}
}

// ---------------------------------------------------------------
// 004 - Create idempotency_records table
// ---------------------------------------------------------------
func migration004CreateIdempotencyRecordsTable() Migration {
	return Migration{
		Version:     4,
		Description: "Create idempotency_records table",
		Up: `
		CREATE TABLE IF NOT EXISTS idempotency_records (
			key        VARCHAR(255) PRIMARY KEY,
			response   TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);`,
		Down: `DROP TABLE IF EXISTS idempotency_records;`,
	}
}

// ---------------------------------------------------------------
// 005 - Add indexes for performance
// ---------------------------------------------------------------
func migration005AddIndexes() Migration {
	return Migration{
		Version:     5,
		Description: "Add indexes for performance",
		Up: `
		CREATE INDEX IF NOT EXISTS idx_tasks_assignee_id     ON tasks(assignee_id);
		CREATE INDEX IF NOT EXISTS idx_tasks_status          ON tasks(status);
		CREATE INDEX IF NOT EXISTS idx_task_logs_task_id     ON task_logs(task_id);
		CREATE INDEX IF NOT EXISTS idx_task_logs_changed_by  ON task_logs(changed_by);
		`,
		Down: `
		DROP INDEX IF EXISTS idx_tasks_assignee_id;
		DROP INDEX IF EXISTS idx_tasks_status;
		DROP INDEX IF EXISTS idx_task_logs_task_id;
		DROP INDEX IF EXISTS idx_task_logs_changed_by;
		`,
	}
}
