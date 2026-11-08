-- saved filters can be owned by a user. Filters without an owner are shared
-- by all users. Existing filters remain shared.
ALTER TABLE `saved_filters` ADD COLUMN `user_id` integer;

DROP INDEX `index_saved_filters_on_mode_name_unique`;
CREATE UNIQUE INDEX `index_saved_filters_on_user_id_mode_name_unique` on `saved_filters` (ifnull(`user_id`, 0), `mode`, `name`);
