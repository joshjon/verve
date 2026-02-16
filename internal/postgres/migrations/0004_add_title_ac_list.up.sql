ALTER TABLE task ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE task ADD COLUMN acceptance_criteria_list TEXT[] NOT NULL DEFAULT '{}';

-- Migrate existing single-string acceptance_criteria into the new array column
UPDATE task SET acceptance_criteria_list = ARRAY[acceptance_criteria]
WHERE acceptance_criteria IS NOT NULL AND acceptance_criteria != '';

ALTER TABLE task DROP COLUMN acceptance_criteria;
