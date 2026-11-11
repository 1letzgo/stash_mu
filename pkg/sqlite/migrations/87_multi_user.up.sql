-- multi-user support: user accounts, and per-user ratings, play and o history,
-- resume position, image o-counters and saved filters. Existing data belongs
-- to the primary user (id 1), which is also the user used when
-- authentication is not configured.

CREATE TABLE `users` (
  `id` integer not null primary key,
  `username` varchar(255) not null,
  `password_hash` varchar(255) not null default '',
  `role` varchar(16) not null default 'VIEWER',
  `api_key` text,
  `ui_config` text,
  `created_at` datetime not null,
  `updated_at` datetime not null,
  `libraries` text
);

CREATE UNIQUE INDEX `index_users_on_username` ON `users` (`username` COLLATE NOCASE);
CREATE UNIQUE INDEX `index_users_on_api_key` ON `users` (`api_key`) WHERE `api_key` IS NOT NULL AND `api_key` != '';

-- play and o history become per-user
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

-- ratings become per-user
CREATE TABLE `scene_ratings` (
  `user_id` integer not null,
  `scene_id` integer not null,
  `rating` tinyint not null,
  PRIMARY KEY (`user_id`, `scene_id`),
  foreign key(`scene_id`) references `scenes`(`id`) on delete CASCADE
);
CREATE INDEX `index_scene_ratings_on_scene_id` ON `scene_ratings` (`scene_id`);
INSERT INTO `scene_ratings` (`user_id`, `scene_id`, `rating`)
  SELECT 1, `id`, `rating` FROM `scenes` WHERE `rating` IS NOT NULL;
ALTER TABLE `scenes` DROP COLUMN `rating`;

CREATE TABLE `image_ratings` (
  `user_id` integer not null,
  `image_id` integer not null,
  `rating` tinyint not null,
  PRIMARY KEY (`user_id`, `image_id`),
  foreign key(`image_id`) references `images`(`id`) on delete CASCADE
);
CREATE INDEX `index_image_ratings_on_image_id` ON `image_ratings` (`image_id`);
INSERT INTO `image_ratings` (`user_id`, `image_id`, `rating`)
  SELECT 1, `id`, `rating` FROM `images` WHERE `rating` IS NOT NULL;
ALTER TABLE `images` DROP COLUMN `rating`;

CREATE TABLE `gallery_ratings` (
  `user_id` integer not null,
  `gallery_id` integer not null,
  `rating` tinyint not null,
  PRIMARY KEY (`user_id`, `gallery_id`),
  foreign key(`gallery_id`) references `galleries`(`id`) on delete CASCADE
);
CREATE INDEX `index_gallery_ratings_on_gallery_id` ON `gallery_ratings` (`gallery_id`);
INSERT INTO `gallery_ratings` (`user_id`, `gallery_id`, `rating`)
  SELECT 1, `id`, `rating` FROM `galleries` WHERE `rating` IS NOT NULL;
ALTER TABLE `galleries` DROP COLUMN `rating`;

CREATE TABLE `performer_ratings` (
  `user_id` integer not null,
  `performer_id` integer not null,
  `rating` tinyint not null,
  PRIMARY KEY (`user_id`, `performer_id`),
  foreign key(`performer_id`) references `performers`(`id`) on delete CASCADE
);
CREATE INDEX `index_performer_ratings_on_performer_id` ON `performer_ratings` (`performer_id`);
INSERT INTO `performer_ratings` (`user_id`, `performer_id`, `rating`)
  SELECT 1, `id`, `rating` FROM `performers` WHERE `rating` IS NOT NULL;
ALTER TABLE `performers` DROP COLUMN `rating`;

CREATE TABLE `studio_ratings` (
  `user_id` integer not null,
  `studio_id` integer not null,
  `rating` tinyint not null,
  PRIMARY KEY (`user_id`, `studio_id`),
  foreign key(`studio_id`) references `studios`(`id`) on delete CASCADE
);
CREATE INDEX `index_studio_ratings_on_studio_id` ON `studio_ratings` (`studio_id`);
INSERT INTO `studio_ratings` (`user_id`, `studio_id`, `rating`)
  SELECT 1, `id`, `rating` FROM `studios` WHERE `rating` IS NOT NULL;
ALTER TABLE `studios` DROP COLUMN `rating`;

CREATE TABLE `group_ratings` (
  `user_id` integer not null,
  `group_id` integer not null,
  `rating` tinyint not null,
  PRIMARY KEY (`user_id`, `group_id`),
  foreign key(`group_id`) references `groups`(`id`) on delete CASCADE
);
CREATE INDEX `index_group_ratings_on_group_id` ON `group_ratings` (`group_id`);
INSERT INTO `group_ratings` (`user_id`, `group_id`, `rating`)
  SELECT 1, `id`, `rating` FROM `groups` WHERE `rating` IS NOT NULL;
ALTER TABLE `groups` DROP COLUMN `rating`;

-- image o-counters become per-user
CREATE TABLE `images_o_counts` (
  `user_id` integer not null,
  `image_id` integer not null,
  `o_counter` integer not null default 0,
  PRIMARY KEY (`user_id`, `image_id`),
  foreign key(`image_id`) references `images`(`id`) on delete CASCADE
);
CREATE INDEX `index_images_o_counts_on_image_id` ON `images_o_counts` (`image_id`);
INSERT INTO `images_o_counts` (`user_id`, `image_id`, `o_counter`)
  SELECT 1, `id`, `o_counter` FROM `images` WHERE `o_counter` > 0;
ALTER TABLE `images` DROP COLUMN `o_counter`;

-- saved filters can be owned by a user. Filters without an owner are shared
-- by all users. Existing filters remain shared.
ALTER TABLE `saved_filters` ADD COLUMN `user_id` integer;

DROP INDEX `index_saved_filters_on_mode_name_unique`;
CREATE UNIQUE INDEX `index_saved_filters_on_user_id_mode_name_unique` on `saved_filters` (ifnull(`user_id`, 0), `mode`, `name`);
