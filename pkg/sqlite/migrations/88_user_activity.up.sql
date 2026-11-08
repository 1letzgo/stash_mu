-- play and o history become per-user. Existing history belongs to the
-- primary user (id 1), which is also the user used when authentication
-- is not configured.
ALTER TABLE `scenes_view_dates` ADD COLUMN `user_id` integer not null default 1;
ALTER TABLE `scenes_o_dates` ADD COLUMN `user_id` integer not null default 1;

CREATE INDEX `index_scenes_view_dates_on_user_id_scene_id` ON `scenes_view_dates` (`user_id`, `scene_id`);
CREATE INDEX `index_scenes_o_dates_on_user_id_scene_id` ON `scenes_o_dates` (`user_id`, `scene_id`);

-- resume time and play duration become per-user
CREATE TABLE `scenes_user_activity` (
  `user_id` integer not null,
  `scene_id` integer not null,
  `resume_time` float not null default 0,
  `play_duration` float not null default 0,
  PRIMARY KEY (`user_id`, `scene_id`),
  foreign key(`scene_id`) references `scenes`(`id`) on delete CASCADE
);

CREATE INDEX `index_scenes_user_activity_on_scene_id` ON `scenes_user_activity` (`scene_id`);

INSERT INTO `scenes_user_activity` (`user_id`, `scene_id`, `resume_time`, `play_duration`)
  SELECT 1, `id`, `resume_time`, `play_duration` FROM `scenes`
  WHERE `resume_time` != 0 OR `play_duration` != 0;

ALTER TABLE `scenes` DROP COLUMN `resume_time`;
ALTER TABLE `scenes` DROP COLUMN `play_duration`;
