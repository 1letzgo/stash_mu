-- ratings become per-user. Existing ratings belong to the primary user (id 1).

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
